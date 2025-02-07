package zfssupport

import (
	"ceph-to-zfs/pkg/ctz/logging"
	"ceph-to-zfs/pkg/ctz/status"
	"fmt"
	"github.com/mistifyio/go-zfs"
	"os/exec"
	"strconv"
	"strings"
)

// ZvolDestination represents an already-prepared Zvol. It should already exist with an appropriate size.
type ZvolDestination struct {
	dataset *zfs.Dataset
}

type ZvolSnapshot struct {
	Name        string
	SnapDataSet *zfs.Dataset
}

func (z *ZvolDestination) Snapshots() ([]*ZvolSnapshot, error) {
	snapshots, err := z.dataset.Snapshots()
	if err != nil {
		return nil, err
	}
	var out []*ZvolSnapshot
	for _, snapshot := range snapshots {
		path := snapshot.Name
		// ZFS snapshot names use the format pool/path/to/dataset@snapname, but we just want the 'snapname'
		parts := strings.Split(path, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("snapshot path %s does not look like a valid zfs snapshot name", path)
		}
		snapName := parts[1]
		out = append(out, &ZvolSnapshot{
			Name:        snapName,
			SnapDataSet: snapshot,
		})
	}
	return out, nil
}

func (z *ZvolDestination) RevertTo(snap *ZvolSnapshot) error {
	err := snap.SnapDataSet.Rollback(true)
	if err != nil {
		return err
	}
	return nil
}

func (z *ZvolDestination) DevNode() (string, error) {
	path := z.dataset.Name
	return fmt.Sprintf("/dev/zvol/%s", path), nil
}

func (z *ZvolDestination) NewSnapshot(name string) (*zfs.Dataset, error) {
	snapshot, err := z.dataset.Snapshot(name, false)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

type ZfsContext struct {
	baseDataset *zfs.Dataset
}

func ZfsContextByPath(path string) (*ZfsContext, error) {
	ds, err := zfs.GetDataset(path)
	if err != nil {
		return nil, err
	}
	return &ZfsContext{baseDataset: ds}, nil
}

// PrepareChild takes a relative path (e.g. if starting at tank/foo, and you want tank/foo/bar, then the name should
// just be "bar"), a size, and a block size, and returns a ZvolDestination appropriate to those parameters. If it does
// not exist, it will be created. If it exists but is too small (e.g. due to expanding the image on the Ceph side),
// it will be expanded. Otherwise, it will be returned as-is. Note that if the image exists, but the block size is
// wrong, no attempt will be made to correct it.
func (z *ZfsContext) PrepareChild(name string, neededSize uint64, prefBlockSize uint64, log *logging.JobStatusLogger) (dest *ZvolDestination, err error) {
	baseName := z.baseDataset.Name

	expectedPath := baseName + "/" + name
	children, err := z.baseDataset.Children(1)
	if err != nil {
		return nil, err
	}
	// Iterate through children until we find one with the name we want.
	log.SetStatus(status.MakeStatus(status.Preparing, "Finding dataset"))
	for _, child := range children {
		path := child.Name
		if path == expectedPath {
			actualSize := child.Volsize
			// TODO: no support for shrinking - how would that even work?
			if actualSize < neededSize {
				log.SetStatus(status.MakeStatus(status.InProgress, fmt.Sprintf("Resizing (%v -> %v)", actualSize, neededSize)))
				err = child.SetProperty("volsize", strconv.FormatUint(neededSize, 10))
				if err != nil {
					return nil, err
				}
			}
			log.SetStatus(status.MakeStatus(status.Success, "Found dataset"))
			return &ZvolDestination{dataset: child}, nil
		}
	}
	log.SetStatus(status.MakeStatus(status.InProgress, "Creating"))
	// Existing dataset not found - need to create
	//props := make(map[string]string)
	// TODO: ceph object size != block size! this is resulting in 4MiB block size instead of 4KiB
	//props["volblocksize"] = strconv.FormatUint(prefBlockSize, 10)
	child, err := createVolume(expectedPath, neededSize)
	if err != nil {
		return nil, err
	}
	log.SetStatus(status.MakeStatus(status.Success, "Created child"))
	return &ZvolDestination{dataset: child}, nil
}

func createVolume(name string, size uint64) (*zfs.Dataset, error) {
	args := make([]string, 5, 6)
	args[0] = "create"
	args[1] = "-p"
	args[2] = "-s"
	args[3] = "-V"
	args[4] = strconv.FormatUint(size, 10)
	args = append(args, name)
	err := exec.Command("zfs", args...).Run()
	if err != nil {
		return nil, err
	}
	return zfs.GetDataset(name)
}
