package backup

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"sync"
)

type TopLevelTask struct {
	cfg      *config.TopLevelProcessedConfig
	log      *logging.JobStatusLogger
	children []*PoolBackupTask
	childMap map[string]*PoolBackupTask
	mt       *task.ManagedTask
}

func NewTopLevelTask(cfg *config.TopLevelProcessedConfig) *TopLevelTask {
	log := logging.NewRootLogger("Main")
	out := &TopLevelTask{
		cfg:      cfg,
		log:      log,
		childMap: make(map[string]*PoolBackupTask),
	}
	out.mt = task.NewManagedTask(log, out.prep, out.run)
	return out
}

func (t *TopLevelTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

func (t *TopLevelTask) Children() []task.Task {
	return util.Map(t.children, func(in *PoolBackupTask) task.Task {
		return in
	})
}

func (t *TopLevelTask) prep() error {
	var children []*PoolBackupTask
	for _, jobCfg := range t.cfg.Jobs {
		child := t.childMap[jobCfg.Label]
		if child == nil {
			child = NewPoolBackupTask(jobCfg, t.log)
			t.childMap[jobCfg.Label] = child
		}
		children = append(children, child)
	}
	t.children = children
	_ = task.RunParallel(t.children, func(bt *PoolBackupTask) error { return bt.Prepare() })
	return nil
}

func (t *TopLevelTask) run() error {
	t.log.SetStatus(status.MakeStatus(status.InProgress, "Running Children"))
	wg := &sync.WaitGroup{}
	_ = task.RunParallel(t.children, func(bt *PoolBackupTask) error { return bt.Run() })
	wg.Wait()
	return nil
}

func (t *TopLevelTask) Run() error {
	return t.mt.Run(nil)
}

func (t *TopLevelTask) Prepare() error {
	return t.mt.Prepare()
}

func (t *TopLevelTask) Label() string {
	return "Root"
}

func (t *TopLevelTask) Id() string {
	return "root"
}

var _ task.Task = &TopLevelTask{}
