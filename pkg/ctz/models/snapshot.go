package models

import "time"

type Snapshot interface {
	Name() string
	When() time.Time
}
