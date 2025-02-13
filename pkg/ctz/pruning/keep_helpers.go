package pruning

import "github.com/mattventura/ceph-to-zfs/pkg/ctz/models"

func filterSnapList(snaps []models.Snapshot, predicate func(models.Snapshot) bool) []models.Snapshot {
	r := make([]models.Snapshot, 0, len(snaps))
	for i := range snaps {
		if predicate(snaps[i]) {
			r = append(r, snaps[i])
		}
	}
	return r
}

func partitionSnapList(snaps []models.Snapshot, predicate func(models.Snapshot) bool) (sTrue, sFalse []models.Snapshot) {
	for i := range snaps {
		if predicate(snaps[i]) {
			sTrue = append(sTrue, snaps[i])
		} else {
			sFalse = append(sFalse, snaps[i])
		}
	}
	return
}

func shallowCopySnapList(snaps []models.Snapshot) []models.Snapshot {
	c := make([]models.Snapshot, len(snaps))
	copy(c, snaps)
	return c
}
