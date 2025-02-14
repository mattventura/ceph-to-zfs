package models

import "time"

type Snapshot interface {
	Name() string
	When() time.Time
}

type ComparableSnapshot interface {
	Snapshot
	comparable
}

type CephSnapshot struct {
	name string
	when time.Time
	Id   uint64
}

func NewCephSnapshot(name string, when time.Time, id uint64) *CephSnapshot {
	return &CephSnapshot{name: name, when: when, Id: id}
}

func (c *CephSnapshot) Name() string {
	return c.name
}

func (c *CephSnapshot) When() time.Time {
	return c.when
}

var _ Snapshot = &CephSnapshot{}
