package backup

import (
	context2 "context"
	"fmt"
	"github.com/ceph/go-ceph/rbd"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/cephsupport"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/zfssupport"
	"golang.org/x/sync/semaphore"
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
	log := parentLog.MakeOrReplaceChild(logging.LoggerKey(jobConfig.Id), false)
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
				tsk = NewImageBackupTask(name, t.cephConfig, t.poolName, zfsContext, t.log, t.jobConfig.Pruning)
				t.childMap[name] = tsk
			}
			children = append(children, tsk)
			included = append(included, name)
		} else {
			excluded = append(excluded, name)
		}
	}

	t.children = children

	if len(children) == 0 {
		t.log.SetStatus(status.MakeStatus(status.Failed, "No images found to back up"))
		return nil
	}

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

	if len(children) == 0 {
		t.log.SetStatus(status.MakeStatus(status.Failed, "No images found to back up"))
		return nil
	}

	t.log.SetStatus(status.MakeStatus(status.InProgress, "Running Children"))
	childrenFailed := 0

	// Wait for all children to finish
	wg := &sync.WaitGroup{}
	// Limit concurrency
	// TODO: move this and put the semaphore logic in the child
	sem := semaphore.NewWeighted(int64(t.jobConfig.MaxConcurrency))

	for _, child := range children {
		wg.Add(1)
		go func() {
			acquire := sem.TryAcquire(1)
			if !acquire {
				child.log.SetStatus(status.MakeStatus(status.Waiting, "Waiting for concurrency limit"))
				err := sem.Acquire(context2.TODO(), 1)
				if err != nil {
					child.log.SetStatus(status.MakeStatus(status.Failed, "Failed to acquire semaphore"))
				}
			}
			defer sem.Release(1)
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
	return t.mt.Run(nil)
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
