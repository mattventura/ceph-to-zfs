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
	// IsBad indicates that the status indicates some sort of failure
	IsBad() bool
}

type statusType struct {
	label      string
	isTerminal bool
	isBad      bool
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

var (
	// predefined statuses

	NotStarted     StatusType = &statusType{label: "Not Started"}
	Preparing      StatusType = &statusType{label: "Preparing"}
	Ready          StatusType = &statusType{label: "Ready"}
	InProgress     StatusType = &statusType{label: "In Progress"}
	Active         StatusType = &statusType{label: "Active"}
	Finishing      StatusType = &statusType{label: "Finishing"}
	Success        StatusType = &statusType{label: "Success", isTerminal: true}
	Failed         StatusType = &statusType{label: "Failed", isTerminal: true, isBad: true}
	Skipped        StatusType = &statusType{label: "Skipped", isTerminal: true}
	ChildrenFailed StatusType = &statusType{label: "Children Failed", isTerminal: true, isBad: true}

	// interface checks
	_ StatusType = &statusType{}
)
