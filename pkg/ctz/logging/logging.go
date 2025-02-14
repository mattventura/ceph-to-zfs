package logging

import (
	"errors"
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"io"
	"sync"
	"time"
)

type LoggerKey string

type logFunc func(path []LoggerKey, msg string)

func defaultLogFunc(path []LoggerKey, msg string) {
	timeFmt := time.Now().Format("2006-01-02 15:04:05.000")
	pathFmt := util.Join(path, " / ")
	fmt.Printf("%v [ %v ]: %v\n", timeFmt, pathFmt, msg)
}

var _ logFunc = defaultLogFunc

// JobStatusLogger is responsible for job status, logging, extra information, and more. It is expected that a task
// will have a JobStatusLogger, that the task will report updates to the JobStatusLogger, and that the task will make
// its JobStatusLogger available for querying.
//
// JSLs are hierarchal - they can have a parent, and children. Apart from a root logger (see NewRootLogger), JSLs are
// typically created by calling MakeOrReplaceChild on another logger.
//
// JSLs should not be copied by value.
type JobStatusLogger struct {
	// Don't copy by value
	_              sync.Mutex
	name           LoggerKey
	parent         *JobStatusLogger
	includeParent  bool
	logFunc        logFunc
	logMsgs        []string
	status         status.Status
	children       map[LoggerKey]*JobStatusLogger
	childLock      *sync.RWMutex
	fixedExtraData map[string]any
	extraData      map[string]any
	extraDataLock  *sync.RWMutex
	detailData     map[string]any
	detailDataLock *sync.RWMutex
}

// NewRootLogger produces a logger with no parent.
func NewRootLogger(name string) *JobStatusLogger {
	return newJobStatusLogger(LoggerKey(name), nil, false, defaultLogFunc, make(map[LoggerKey]*JobStatusLogger))
}

func newJobStatusLogger(name LoggerKey, parent *JobStatusLogger, includeParent bool, logFunc logFunc, children map[LoggerKey]*JobStatusLogger) *JobStatusLogger {
	if logFunc == nil {
		logFunc = defaultLogFunc
	}
	return &JobStatusLogger{
		name:           name,
		parent:         parent,
		includeParent:  includeParent,
		logFunc:        logFunc,
		children:       util.CopyMap(children),
		status:         status.SimpleStatus(status.NotStarted),
		extraData:      make(map[string]any),
		fixedExtraData: make(map[string]any),
		detailData:     make(map[string]any),
		childLock:      &sync.RWMutex{},
		extraDataLock:  &sync.RWMutex{},
		detailDataLock: &sync.RWMutex{},
	}
}

// Path is the parental chain of this logger, for display purposes. If includeParent is false, then this is considered
// to be a root logger, and no further parents will be included.
func (l *JobStatusLogger) Path() []*JobStatusLogger {
	if l.parent == nil || !l.includeParent {
		return []*JobStatusLogger{l}
	} else {
		return append(l.parent.Path(), l)
	}
}

// PathKeys is like Path, but only the keys are returned.
func (l *JobStatusLogger) PathKeys() []LoggerKey {
	if l.parent == nil || !l.includeParent {
		return []LoggerKey{l.name}
	} else {
		return append(l.parent.PathKeys(), l.name)
	}
}

// PathFormatted is like PathKeys, except a pre-formatted string is returned instead of the actual list.
func (l *JobStatusLogger) PathFormatted() string {
	return util.Join(l.PathKeys(), " : ")
}

// Children returns a map of children, where the keys are each child's key, and the value is a pointer to the child.
func (l *JobStatusLogger) Children() map[LoggerKey]*JobStatusLogger {
	l.childLock.RLock()
	defer l.childLock.RUnlock()
	return util.CopyMap(l.children)
}

// MakeOrReplaceChild will create or replace an existing child by that name. If includeParent is false, then the new
// logger will still be considered a child of this logger from this logger's standpoint, but the child will consider
// itself to be unparented (i.e. a root logger).
func (l *JobStatusLogger) MakeOrReplaceChild(name LoggerKey, includeParent bool) *JobStatusLogger {
	newChild := newJobStatusLogger(name, l, includeParent, l.logFunc, make(map[LoggerKey]*JobStatusLogger))
	l.childLock.Lock()
	defer l.childLock.Unlock()
	l.children[name] = newChild
	return newChild
}

// Log records a message and outputs it to the console.
func (l *JobStatusLogger) Log(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	l.log(formatted)
}

// Warn records a message and outputs it to the console.
// TODO: make warn/error distinct
func (l *JobStatusLogger) Warn(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	l.log(formatted)
}

func (l *JobStatusLogger) log(formatted string) {
	l.logMsgs = append(l.logMsgs, formatted)
	l.logFunc(l.PathKeys(), formatted)
}

// Status returns the last-reported Status of the task.
func (l *JobStatusLogger) Status() status.Status {
	return l.status
}

// SetSimpleStatus sets a status using only a status.StatusType, where the message is set to the status's label.
func (l *JobStatusLogger) SetSimpleStatus(st status.StatusType) {
	l.SetStatus(status.SimpleStatus(st))
}

// SetStatus sets a custom status. See status.MakeStatus.
func (l *JobStatusLogger) SetStatus(newStatus status.Status) {
	var statusPart string
	if l.status.Type() != newStatus.Type() {
		statusPart = fmt.Sprintf("%v -> %v", l.status.Type().Label(), newStatus.Type().Label())
	} else {
		statusPart = newStatus.Type().Label()
	}
	l.Log("%v: %v", statusPart, newStatus.Msg())
	l.status = newStatus
	if newStatus.Type().IsTerminal() {
		for _, childLogger := range l.Children() {
			if childLogger.status.Type() == status.NotStarted {
				childLogger.SetStatus(status.SimpleStatus(status.Skipped))
			}
		}
	}
}

// SetStatusByError when called with a nil argument is equivalent to SetStatus(status.SimpleStatus(status.Success)).
// When called with a non-nil argument, it is equivalent to SetStatus(status.MakeStatus(status.Failed, err.Error()))
func (l *JobStatusLogger) SetStatusByError(err error) {
	if err != nil {
		l.SetStatus(status.MakeStatus(status.Failed, err.Error()))
	} else {
		l.SetStatus(status.SimpleStatus(status.Success))
	}
}

// SetFinished sets this status to status.Success, along with an optional success message, unless any children have
// failed, in which case it sets the status to status.ChildrenFailed.
func (l *JobStatusLogger) SetFinished(successMsg string) error {
	failedChildren := 0
	for _, child := range l.Children() {
		childStatusType := child.status.Type()
		if childStatusType.IsTerminal() && childStatusType.IsBad() {
			failedChildren++
		}
	}
	if failedChildren > 0 {
		childFailMsg := fmt.Sprintf("%v children failed", failedChildren)
		l.SetStatus(status.MakeStatus(status.ChildrenFailed, childFailMsg))
		// TODO: replace this with errors.Join with the actual messages
		return errors.New(childFailMsg)
	} else {
		if successMsg == "" {
			l.SetSimpleStatus(status.Success)
		} else {
			l.SetStatus(status.MakeStatus(status.Success, successMsg))
		}
		return nil
	}

}

// SimpleRun takes a single function and runs it. It sets the status to status.InProgress before running it, and either
// status.Success or status.Failed depending on whether the function returned an error or not.
func (l *JobStatusLogger) SimpleRun(f func() error) error {
	l.SetSimpleStatus(status.InProgress)
	err := f()
	if err != nil {
		l.SetStatusByError(err)
	} else {
		l.SetSimpleStatus(status.Success)
	}
	return err
}

// ResetData clears all data related to a specific invocation of the task, i.e. anything set by SetExtraData or
// SetDetailData, but not SetFixedExtraData.
func (l *JobStatusLogger) ResetData() {
	l.extraDataLock.Lock()
	defer l.extraDataLock.Unlock()
	l.extraData = make(map[string]any)
	l.detailDataLock.Lock()
	defer l.detailDataLock.Unlock()
	l.detailData = make(map[string]any)
}

// SetFixedExtraData sets values in the extraData map (see GetExtraData), but these are not cleared on a reset. It is
// intended to be used after initializing the logger rather than during an actual run.
func (l *JobStatusLogger) SetFixedExtraData(key string, value any) {
	l.extraDataLock.Lock()
	defer l.extraDataLock.Unlock()
	l.fixedExtraData[key] = value
}

// SetExtraData attaches some extra data to this logger. Values set here override any values set by SetFixedExtraData.
func (l *JobStatusLogger) SetExtraData(key string, value any) {
	l.extraDataLock.Lock()
	defer l.extraDataLock.Unlock()
	l.extraData[key] = value
}

// GetExtraData returns all data set by SetFixedExtraData and SetExtraData.
func (l *JobStatusLogger) GetExtraData() map[string]any {
	l.extraDataLock.RLock()
	defer l.extraDataLock.RUnlock()
	return util.MergeMaps(l.fixedExtraData, l.extraData)
}

// SetDetailData is a but like SetExtraData, but this is a separate map which is intended to be for "heavy" data
// which would be inconveniently large for the normal extra data. The default web UI effectively invokes GetExtraData
// for every task once per second, so any data too large for that use case should go here, as the web UI only polls
// this data for the currently-selected task.
func (l *JobStatusLogger) SetDetailData(key string, value any) {
	l.detailDataLock.Lock()
	defer l.detailDataLock.Unlock()
	l.detailData[key] = value
}

// GetDetailData retrieves all data set by SetDetailData
func (l *JobStatusLogger) GetDetailData() map[string]any {
	l.detailDataLock.RLock()
	defer l.detailDataLock.RUnlock()
	return util.CopyMap(l.detailData)
}

type asWriter struct {
	log *JobStatusLogger
}

func (w *asWriter) Write(b []byte) (n int, err error) {
	w.log.Log(string(b))
	return len(b), nil
}

func (l *JobStatusLogger) AsWriter() io.Writer {
	return &asWriter{log: l}
}
