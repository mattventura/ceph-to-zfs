package logging

import (
	"ceph-to-zfs/pkg/ctz/status"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

type Loggable interface {
}

type logFunc func(path []LoggerKey, msg string)

func Join[K ~string](elems []K, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return string(elems[0])
	}

	var n int
	if len(sep) > 0 {
		if len(sep) >= math.MaxInt/(len(elems)-1) {
			panic("strings: Join output length overflow")
		}
		n += len(sep) * (len(elems) - 1)
	}
	for _, elem := range elems {
		if len(elem) > math.MaxInt-n {
			panic("strings: Join output length overflow")
		}
		n += len(elem)
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(string(elems[0]))
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(string(s))
	}
	return b.String()
}

func defaultLogFunc(path []LoggerKey, msg string) {
	timeFmt := time.Now().Format("2006-01-02 15:04:05.123")
	pathFmt := Join(path, " / ")
	fmt.Printf("%v [ %v ]: %v\n", timeFmt, pathFmt, msg)
}

var (
	_ logFunc = defaultLogFunc
)

type LoggerKey string

type JobStatusLogger struct {
	name          LoggerKey
	parent        *JobStatusLogger
	includeParent bool
	logFunc       logFunc
	logMsgs       []string
	children      map[LoggerKey]*JobStatusLogger
	status        status.Status
	extraData     map[string]any
}

func NewRootLogger(name string) *JobStatusLogger {
	return newJobStatusLogger(LoggerKey(name), nil, false, defaultLogFunc, make(map[LoggerKey]*JobStatusLogger))
}

func newJobStatusLogger(name LoggerKey, parent *JobStatusLogger, includeParent bool, logFunc logFunc, children map[LoggerKey]*JobStatusLogger) *JobStatusLogger {
	if logFunc == nil {
		logFunc = defaultLogFunc
	}
	return &JobStatusLogger{
		name:          name,
		parent:        parent,
		includeParent: includeParent,
		logFunc:       logFunc,
		children:      children,
		status:        status.SimpleStatus(status.NotStarted),
		extraData:     make(map[string]any),
	}
}

func (l *JobStatusLogger) Path() []*JobStatusLogger {
	if l.parent == nil || !l.includeParent {
		return []*JobStatusLogger{l}
	} else {
		return append(l.parent.Path(), l)
	}
}

func (l *JobStatusLogger) PathKeys() []LoggerKey {
	if l.parent == nil || !l.includeParent {
		return []LoggerKey{l.name}
	} else {
		return append(l.parent.PathKeys(), l.name)
	}
}

func (l *JobStatusLogger) PathFormatted() string {
	return Join(l.PathKeys(), " : ")
}

func (l *JobStatusLogger) Children() map[LoggerKey]*JobStatusLogger {
	return l.children
}

func (l *JobStatusLogger) MakeOrReplaceChild(name LoggerKey, includeParent bool) *JobStatusLogger {
	newChild := newJobStatusLogger(name, l, includeParent, l.logFunc, make(map[LoggerKey]*JobStatusLogger))
	l.children[name] = newChild
	return newChild
}

func (l *JobStatusLogger) Log(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	l.log(formatted)
}

func (l *JobStatusLogger) log(formatted string) {
	l.logMsgs = append(l.logMsgs, formatted)
	l.logFunc(l.PathKeys(), formatted)
}

func (l *JobStatusLogger) Status() status.Status {
	return l.status
}

func (l *JobStatusLogger) SetSimpleStatus(st status.StatusType) {
	l.SetStatus(status.SimpleStatus(st))
}

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

func (l *JobStatusLogger) SetStatusByError(err error) {
	if err != nil {
		l.SetStatus(status.MakeStatus(status.Failed, err.Error()))
	} else {
		l.SetStatus(status.SimpleStatus(status.Success))
	}
}

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

func (l *JobStatusLogger) ResetData() {
	l.extraData = make(map[string]any)
}

func (l *JobStatusLogger) SetExtraData(key string, value any) {
	l.extraData[key] = value
}

func (l *JobStatusLogger) GetExtraData() map[string]any {
	return l.extraData
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
