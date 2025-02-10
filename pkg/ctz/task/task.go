package task

import (
	"ceph-to-zfs/pkg/ctz/logging"
	"ceph-to-zfs/pkg/ctz/status"
	"errors"
	"fmt"
	"sync"
)

type Task interface {
	Id() string
	Label() string
	Run() error
	Children() []Task
	StatusLog() *logging.JobStatusLogger
}

type PreparableTask interface {
	Task
	Prepare() error
}

type CancellableTask interface {
	Task
	Cancel() error
}

var InProgressError = errors.New("task is already in progress")

type ManagedTask struct {
	mut      *sync.Mutex
	prepped  bool
	log      *logging.JobStatusLogger
	prepFunc func() error
	taskFunc func() error
}

func NewManagedTask(logger *logging.JobStatusLogger, prepFunc func() error, taskFunc func() error) *ManagedTask {
	return &ManagedTask{log: logger, prepFunc: prepFunc, taskFunc: taskFunc, mut: &sync.Mutex{}}
}
func (mt *ManagedTask) doPrep() error {
	mt.log.SetSimpleStatus(status.Preparing)
	err := mt.prepFunc()
	if err != nil {
		return err
	}
	mt.prepped = true
	mt.log.SetSimpleStatus(status.Ready)
	return nil
}

func (mt *ManagedTask) Prepare() (err error) {
	defer func() {
		if err != nil {
			mt.log.SetStatus(status.MakeStatus(status.Failed, err.Error()))
		}
	}()
	locked := mt.mut.TryLock()
	if !locked {
		return InProgressError
	}
	defer mt.mut.Unlock()
	return mt.doPrep()
}

func (mt *ManagedTask) Run(successMsg func() string) (err error) {
	locked := mt.mut.TryLock()
	if !locked {
		// TODO: we don't really want it to complain if you request it to run
		// while it is explicitly preparing
		return InProgressError
	}
	defer mt.mut.Unlock()
	defer func() {
		// Clear the prepared flag after running this once
		mt.prepped = false
	}()
	defer func() {
		if err != nil {
			mt.log.SetStatusByError(err)
		}
	}()
	if !mt.prepped {
		err := mt.doPrep()
		if err != nil {
			return err
		}
	}
	err = mt.taskFunc()
	if err != nil {
		return err
	}
	// TODO: msg configuration
	if successMsg != nil {
		err = mt.log.SetFinished(successMsg())
	} else {
		err = mt.log.SetFinished("")
	}
	return err
}

func RunParallel[T Task](children []T, f func(T) error) []error {
	var errs []error
	wg := sync.WaitGroup{}
	for _, child := range children {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				rec := recover()
				if rec != nil {
					child.StatusLog().SetStatus(status.MakeStatus(status.Failed, fmt.Sprintf("Recovered from panic: %v", rec)))
				}
			}()
			childErr := f(child)
			if childErr != nil {
				// Ignore error if the child is already in progress
				if !errors.Is(childErr, InProgressError) {
					errs = append(errs, childErr)
				}
			}
		}()
	}
	wg.Wait()
	return errs
}
