package backup

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"sync"
	"time"
)

type TopLevelTask struct {
	cfg      *config.TopLevelProcessedConfig
	log      *logging.JobStatusLogger
	children []*RbdPoolBackupTask
	childMap map[string]*RbdPoolBackupTask
	mt       *task.ManagedTask
}

func NewTopLevelTask(cfg *config.TopLevelProcessedConfig) (*TopLevelTask, error) {
	log := logging.NewRootLogger("Main")
	out := &TopLevelTask{
		cfg:      cfg,
		log:      log,
		childMap: make(map[string]*RbdPoolBackupTask),
	}
	out.mt = task.NewManagedTask(log, out.prep, out.run)
	t := out
	var children []*RbdPoolBackupTask
	var sched gocron.Scheduler
	cronEnabled := !cfg.Globals.DisableAllCron
	if cronEnabled {
		var err error
		sched, err = gocron.NewScheduler(gocron.WithLocation(time.Local))
		if err != nil {
			return nil, util.Wrap("error creating scheduler", err)
		}
	} else {
		log.Log("cron globally disabled")
	}
	// This technically didn't have to move, since it has the childMap
	for _, jobCfg := range t.cfg.Jobs {
		child := t.childMap[jobCfg.Label]
		if child == nil {
			child = NewRbdPoolBackupTask(jobCfg, t.log)
			t.childMap[jobCfg.Label] = child
			if jobCfg.Cron != nil {
				cj := gocron.CronJob(*jobCfg.Cron, false)
				if cronEnabled {
					_, err := sched.NewJob(cj, gocron.NewTask(func() {
						child.log.Log("job triggered by cron '%v'", *jobCfg.Cron)
						s := child.log.Status()
						childStatusType := s.Type()
						// Only run child if it is not active
						if childStatusType == status.Ready || childStatusType.IsTerminal() {
							child.Run()
						} else {
							child.log.Log("skipping cron: job status is currently '%v'", s)
						}
					}))
					if err != nil {
						return nil, err
					}
				}
			}
		}
		children = append(children, child)
		// TODO: need way to disable this when oneshot mode is enabled
	}
	if cronEnabled {
		sched.Start()
	}
	t.children = children
	// TODO: when we have individual sub-jobs running independently of the parent, how do we adequately convey the
	// status of the sub-jobs?
	return out, nil
}

func (t *TopLevelTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

func (t *TopLevelTask) Children() []task.Task {
	return util.Map(t.children, func(in *RbdPoolBackupTask) task.Task {
		return in
	})
}

func (t *TopLevelTask) prep() error {
	_ = task.RunParallel(t.children, func(bt *RbdPoolBackupTask) error { return bt.Prepare() })
	return nil
}

func (t *TopLevelTask) run() error {
	t.log.SetStatus(status.MakeStatus(status.InProgress, "Running Children"))
	wg := &sync.WaitGroup{}
	_ = task.RunParallel(t.children, func(bt *RbdPoolBackupTask) error { return bt.Run() })
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
