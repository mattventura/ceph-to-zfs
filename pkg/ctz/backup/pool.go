package backup

import (
	"ceph-to-zfs/pkg/ctz/cephsupport"
	"ceph-to-zfs/pkg/ctz/config"
	"ceph-to-zfs/pkg/ctz/logging"
	"ceph-to-zfs/pkg/ctz/status"
	"ceph-to-zfs/pkg/ctz/task"
	"ceph-to-zfs/pkg/ctz/util"
	"ceph-to-zfs/pkg/ctz/zfssupport"
	"fmt"
	"github.com/ceph/go-ceph/rbd"
	"runtime"
	"sync"
)

type PoolBackupTask struct {
	cephConfig *config.CephClusterConfig
	jobConfig  *config.PoolJobProcessedConfig
	poolName   string
	log        *logging.JobStatusLogger
	children   []*ImageBackupTask
	childMap   map[string]*ImageBackupTask
	mt         *task.ManagedTask
}

func NewPoolBackupTask(
	jobConfig *config.PoolJobProcessedConfig,
	parentLog *logging.JobStatusLogger,
) *PoolBackupTask {
	log := parentLog.MakeOrReplaceChild(logging.LoggerKey(jobConfig.Label), false)
	out := &PoolBackupTask{
		cephConfig: jobConfig.ClusterConfig,
		jobConfig:  jobConfig,
		poolName:   jobConfig.CephPoolName,
		log:        log,
		children:   []*ImageBackupTask{},
		childMap:   map[string]*ImageBackupTask{},
	}
	out.mt = task.NewManagedTask(log, out.prep, out.run)
	return out
}

func (t *PoolBackupTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

func (t *PoolBackupTask) Children() []task.Task {
	return util.Map(t.children, func(in *ImageBackupTask) task.Task {
		return in
	})
}

func (t *PoolBackupTask) includeImage(name string) bool {
	j := t.jobConfig
	if j.ImageIncludeRegex == nil {
		return true
	} else {
		return j.ImageIncludeRegex.MatchString(name)
	}
}

func (t *PoolBackupTask) excludeImage(name string) bool {
	j := t.jobConfig
	if j.ImageExcludeRegex == nil {
		return false
	} else {
		return j.ImageExcludeRegex.MatchString(name)
	}
}

func (t *PoolBackupTask) shouldBackupImage(name string) bool {
	// if only "include" is specified, only things matching the include pattern are included
	// if only "exclude" is specified, then only things not matching the exclude pattern are included
	// if both are specified, then items must match the include pattern *and not* match the exclude pattern
	if t.excludeImage(name) {
		return false
	}
	return t.includeImage(name)
}

// prep contains only the
func (t *PoolBackupTask) prep() (err error) {
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Connecting to Ceph Cluster"))
	conn, err := cephsupport.Connect(t.cephConfig)
	if err != nil {
		return err
	}
	// TODO: double connection
	defer func() { go conn.Shutdown() }()
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Opening IOContext"))
	context, err := conn.OpenIOContext(t.poolName)
	if err != nil {
		return err
	}
	defer func() { go context.Destroy() }()
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Enumerating Images"))
	names, err := rbd.GetImageNames(context)
	if err != nil {
		return err
	}
	zfsContext, err := zfssupport.ZfsContextByPath(t.jobConfig.ZfsDestination)
	if err != nil {
		return err
	}
	var children []*ImageBackupTask
	var included []string
	var excluded []string
	for _, name := range names {
		if t.shouldBackupImage(name) {
			t.log.Log("Image %v included", name)
			tsk := t.childMap[name]
			if tsk == nil {
				tsk = NewImageBackupTask(name, t.cephConfig, t.poolName, zfsContext, t.log)
				t.childMap[name] = tsk
			}
			children = append(children, tsk)
			included = append(included, name)
		} else {
			excluded = append(excluded, name)
		}
	}
	t.children = children
	t.log.Log("Included: %v", included)
	t.log.Log("Excluded: %v", excluded)
	return nil
}

func (t *PoolBackupTask) run() (err error) {
	// TODO: needed?
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	conn, err := cephsupport.Connect(t.cephConfig)
	if err != nil {
		return err
	}
	defer func() { go conn.Shutdown() }()
	context, err := conn.OpenIOContext(t.poolName)
	if err != nil {
		return err
	}
	defer func() { go context.Destroy() }()

	children := t.children
	t.log.SetStatus(status.MakeStatus(status.InProgress, "Running Children"))
	childrenFailed := 0
	wg := &sync.WaitGroup{}
	for _, child := range children {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				rec := recover()
				if rec != nil {
					childrenFailed++
					child.log.SetStatus(status.MakeStatus(status.Failed, fmt.Sprintf("Recovered from panic: %v", rec)))
				}
			}()
			childErr := child.Run()
			if childErr != nil {
				childrenFailed++
			}
		}()
	}
	wg.Wait()
	return err
}

func (t *PoolBackupTask) Run() error {
	return t.mt.Run()
}

func (t *PoolBackupTask) Prepare() (err error) {
	return t.mt.Prepare()
}

func (t *PoolBackupTask) Id() string {
	return t.jobConfig.Id
}

func (t *PoolBackupTask) Label() string {
	return t.jobConfig.Label
}

var _ task.PreparableTask = &PoolBackupTask{}
