package pruning

import (
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning/retentiongrid"
	"github.com/pkg/errors"
	"regexp"
	"time"
)

// KeepGrid fits snapshots that match a given regex into a retentiongrid.Grid,
// uses the most recent snapshot among those that match the regex as 'now',
// and deletes all snapshots that do not fit the grid specification.
type KeepGrid[T models.Snapshot] struct {
	retentionGrid *retentiongrid.Grid
	re            *regexp.Regexp
}

func NewKeepGrid[T models.Snapshot](in *PruneGrid) (p *KeepGrid[T], err error) {

	if in.Regex == "" {
		return nil, fmt.Errorf("Regex must not be empty")
	}
	re, err := regexp.Compile(in.Regex)
	if err != nil {
		return nil, errors.Wrap(err, "Regex is invalid")
	}

	return newKeepGrid[T](re, in.Grid)
}

func MustNewKeepGrid[T models.Snapshot](regex, gridspec string) *KeepGrid[T] {

	ris, err := ParseRetentionIntervalSpec(gridspec)
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(regex)

	grid, err := newKeepGrid[T](re, ris)
	if err != nil {
		panic(err)
	}
	return grid
}

func newKeepGrid[T models.Snapshot](re *regexp.Regexp, configIntervals []RetentionInterval) (*KeepGrid[T], error) {
	if re == nil {
		panic("re must not be nil")
	}

	if len(configIntervals) == 0 {
		return nil, errors.New("retention grid must specify at least one interval")
	}

	intervals := make([]retentiongrid.Interval, len(configIntervals))
	for i := range configIntervals {
		intervals[i] = &configIntervals[i]
	}

	// Assert intervals are of increasing length (not necessarily required, but indicates config mistake)
	lastDuration := time.Duration(0)
	for i := range intervals {

		if intervals[i].Length() < lastDuration {
			// If all intervals before were keep=all, this is ok
			allPrevKeepCountAll := true
			for j := i - 1; allPrevKeepCountAll && j >= 0; j-- {
				allPrevKeepCountAll = intervals[j].KeepCount() == retentiongrid.RetentionGridKeepCountAll
			}
			if allPrevKeepCountAll {
				goto isMonotonicIncrease
			}
			return nil, errors.New("retention grid interval length must be monotonically increasing")
		}
	isMonotonicIncrease:
		lastDuration = intervals[i].Length()
	}

	return &KeepGrid[T]{
		retentionGrid: retentiongrid.NewGrid(intervals),
		re:            re,
	}, nil
}

// Prune filters snapshots with the retention grid.
func (p *KeepGrid[T]) KeepRule(snaps []T) (destroyList []T) {

	matching, notMatching := partitionSnapList(snaps, func(snapshot T) bool {
		return p.re.MatchString(snapshot.Name())
	})

	// snaps that don't match the regex are not kept by this rule
	destroyList = append(destroyList, notMatching...)

	if len(matching) == 0 {
		return destroyList
	}

	// Evaluate retention grid
	entrySlice := make([]retentiongrid.Entry, 0)
	for i := range matching {
		entrySlice = append(entrySlice, matching[i])
	}
	_, gridDestroyList := p.retentionGrid.FitEntries(entrySlice)

	// Revert adaptors
	for i := range gridDestroyList {
		destroyList = append(destroyList, gridDestroyList[i].(T))
	}
	return destroyList
}
