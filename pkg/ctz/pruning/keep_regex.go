package pruning

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"regexp"
)

type KeepRegex[T models.Snapshot] struct {
	expr   *regexp.Regexp
	negate bool
}

var _ KeepRule[models.Snapshot] = &KeepRegex[models.Snapshot]{}

func NewKeepRegex[T models.Snapshot](expr string, negate bool) (*KeepRegex[T], error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	return &KeepRegex[T]{re, negate}, nil
}

func MustKeepRegex[T models.Snapshot](expr string, negate bool) *KeepRegex[T] {
	k, err := NewKeepRegex[T](expr, negate)
	if err != nil {
		panic(err)
	}
	return k
}

func (k *KeepRegex[T]) KeepRule(snaps []T) []T {
	return filterSnapList(snaps, func(s T) bool {
		if k.negate {
			return k.expr.FindStringIndex(s.Name()) != nil
		} else {
			return k.expr.FindStringIndex(s.Name()) == nil
		}
	})
}
