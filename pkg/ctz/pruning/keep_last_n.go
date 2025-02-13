package pruning

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

type KeepLastN struct {
	n  int
	re *regexp.Regexp
}

var _ KeepRule = &KeepLastN{}

func MustKeepLastN(n int, regex string) *KeepLastN {
	k, err := NewKeepLastN(n, regex)
	if err != nil {
		panic(err)
	}
	return k
}

func NewKeepLastN(n int, regex string) (*KeepLastN, error) {
	if n <= 0 {
		return nil, errors.Errorf("must specify positive number as 'keep last count', got %d", n)
	}
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, errors.Errorf("invalid regex %q: %s", regex, err)
	}
	return &KeepLastN{n, re}, nil
}

func (k KeepLastN) KeepRule(snaps []models.Snapshot) (destroyList []models.Snapshot) {
	matching, notMatching := partitionSnapList(snaps, func(snapshot models.Snapshot) bool {
		return k.re.MatchString(snapshot.Name())
	})
	// snaps that don't match the regex are not kept by this rule
	destroyList = append(destroyList, notMatching...)

	if len(matching) == 0 {
		return destroyList
	}

	sort.Slice(matching, func(i, j int) bool {
		// by date (youngest first)
		id, jd := matching[i].When(), matching[j].When()
		if !id.Equal(jd) {
			return id.After(jd)
		}
		// then lexicographically descending (e.g. b, a)
		return strings.Compare(matching[i].Name(), matching[j].Name()) == 1
	})

	n := k.n
	if n > len(matching) {
		n = len(matching)
	}
	destroyList = append(destroyList, matching[n:]...)
	return destroyList
}
