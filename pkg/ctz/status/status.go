package status

type Status interface {
	Type() StatusType
	Msg() string
}

func MakeStatus(st StatusType, msg string) Status {
	return &status{st, msg}
}

func SimpleStatus(st StatusType) Status {
	return MakeStatus(st, st.Label())
}

type status struct {
	statusType StatusType
	msg        string
}

func (s *status) Type() StatusType {
	return s.statusType
}

func (s *status) Msg() string {
	return s.msg
}

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

func (s *statusType) Label() string {
	return s.label
}

func (s *statusType) IsTerminal() bool {
	return s.isTerminal
}

func (s *statusType) IsBad() bool {
	return s.isBad
}

func (s *statusType) IsActive() bool {
	return s.isActive
}

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

	// interface checks
	_ StatusType = &statusType{}
)
