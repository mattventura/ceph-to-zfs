package pruning

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"regexp"
)

type KeepRegex struct {
	expr   *regexp.Regexp
	negate bool
}

var _ KeepRule = &KeepRegex{}

func NewKeepRegex(expr string, negate bool) (*KeepRegex, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	return &KeepRegex{re, negate}, nil
}

func MustKeepRegex(expr string, negate bool) *KeepRegex {
	k, err := NewKeepRegex(expr, negate)
	if err != nil {
		panic(err)
	}
	return k
}

func (k *KeepRegex) KeepRule(snaps []models.Snapshot) []models.Snapshot {
	return filterSnapList(snaps, func(s models.Snapshot) bool {
		if k.negate {
			return k.expr.FindStringIndex(s.Name()) != nil
		} else {
			return k.expr.FindStringIndex(s.Name()) == nil
		}
	})
}
