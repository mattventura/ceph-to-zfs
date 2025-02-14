package pruning

import "github.com/mattventura/ceph-to-zfs/pkg/ctz/models"

func filterSnapList[T models.Snapshot](snaps []T, predicate func(T) bool) []T {
	r := make([]T, 0, len(snaps))
	for i := range snaps {
		if predicate(snaps[i]) {
			r = append(r, snaps[i])
		}
	}
	return r
}

func partitionSnapList[T models.Snapshot](snaps []T, predicate func(T) bool) (sTrue, sFalse []T) {
	for i := range snaps {
		if predicate(snaps[i]) {
			sTrue = append(sTrue, snaps[i])
		} else {
			sFalse = append(sFalse, snaps[i])
		}
	}
	return
}

func shallowCopySnapList[T models.Snapshot](snaps []T) []T {
	c := make([]T, len(snaps))
	copy(c, snaps)
	return c
}
