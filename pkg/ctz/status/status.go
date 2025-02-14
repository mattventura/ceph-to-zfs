package status

import "fmt"

// Status represents the current status of a job. The Type represents a general type of status, which has properties
// such as whether it indicates a failure, or whether it is terminal vs still in progress.
type Status interface {
	fmt.Stringer
	Type() StatusType
	Msg() string
}

// MakeStatus produces a Status from a StatusType and user-readable message
func MakeStatus(st StatusType, msg string) Status {
	return &status{st, msg}
}

// SimpleStatus produces a Status from a StatusType, using the default Label for that StatusType as the message
func SimpleStatus(st StatusType) Status {
	return MakeStatus(st, st.Label())
}

type status struct {
	statusType StatusType
	msg        string
}

// String converts a Status into a string of the form StatusType(StatusMessage)
func (s *status) String() string {
	return fmt.Sprintf("%v(%v)", s.statusType.Label(), s.msg)
}

// Type is the general Type of this status
func (s *status) Type() StatusType {
	return s.statusType
}

// Msg is the custom message for this status
func (s *status) Msg() string {
	return s.msg
}

// StatusType represents a general type of status
type StatusType interface {
	// Label is a human-readable, concise label for the task
	Label() string
	// IsTerminal indicates that the task is no longer processing anything. The status will
	// not change until something triggers a new run of the task.
	IsTerminal() bool
	// IsBad indicates that the status represents some sort of failure.
	IsBad() bool
	// IsActive indicates that the task is actively doing something.
	IsActive() bool
}

type statusType struct {
	label      string
	isTerminal bool
	isBad      bool
	isActive   bool
}

// Label is a human-readable label for this status
func (s *statusType) Label() string {
	return s.label
}

// IsTerminal indicates that the task has completed (successfully or otherwise), and that no more status type
// transitions will occur without outside intervention.
func (s *statusType) IsTerminal() bool {
	return s.isTerminal
}

// IsBad indicates that the task has experienced a failure.
func (s *statusType) IsBad() bool {
	return s.isBad
}

// IsActive indicates that the task is actively doing something.
func (s *statusType) IsActive() bool {
	return s.isActive
}

// TODO: add CanStart() bool

var (
	// predefined statuses

	// Job has not started at all
	NotStarted StatusType = &statusType{label: "Not Started"}
	// Waiting on another job or concurrency limit
	Waiting StatusType = &statusType{label: "Waiting"}
	// Preparing indicates that the job is doing some prep work or enumerating children.
	Preparing  StatusType = &statusType{label: "Preparing", isActive: true}
	Ready      StatusType = &statusType{label: "Ready"}
	InProgress StatusType = &statusType{label: "In Progress", isActive: true}
	Active     StatusType = &statusType{label: "Active", isActive: true}
	Finishing  StatusType = &statusType{label: "Finishing", isActive: true}
	Success    StatusType = &statusType{label: "Success", isTerminal: true}
	Failed     StatusType = &statusType{label: "Failed", isTerminal: true, isBad: true}
	// Skipped indicates that the job never started, but its parent finished
	Skipped StatusType = &statusType{label: "Skipped", isTerminal: true}
	// ChildrenFailed indicates that regardless of whether the job was successful, one or more children failed.
	ChildrenFailed StatusType = &statusType{label: "Children Failed", isTerminal: true, isBad: true}

	// TODO: status for no children when children were expected? or a toggle to allow this to fail

	// interface checks
	_ StatusType = &statusType{}
)
