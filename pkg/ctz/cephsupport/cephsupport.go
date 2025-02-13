package cephsupport

import (
	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"time"
)

type CephImageView struct {
	image *rbd.Image
}

type CephSnapshot struct {
	name string
	when time.Time
}

func (c *CephSnapshot) Name() string {
	return c.name
}

func (c *CephSnapshot) When() time.Time {
	return c.when
}

var _ models.Snapshot = &CephSnapshot{}

func (i *CephImageView) Name() string {
	return i.image.GetName()
}

func (i *CephImageView) Size() (uint64, error) {
	return i.image.GetSize()
}

func (i *CephImageView) SnapNames() ([]string, error) {
	snaps, err := i.image.GetSnapshotNames()
	if err != nil {
		return nil, err
	}
	snapNames := util.Map(snaps, func(in rbd.SnapInfo) string {
		return in.Name
	})
	return snapNames, nil
}

func (i *CephImageView) Snapshots() ([]*CephSnapshot, error) {
	snaps, err := i.image.GetSnapshotNames()
	if err != nil {
		return nil, err
	}
	out := make([]*CephSnapshot, len(snaps))
	// TODO: this is slow to do in serial
	for j, snap := range snaps {
		timestamp, err := i.image.GetSnapTimestamp(snap.Id)
		if err != nil {
			return nil, err
		}
		out[j] = &CephSnapshot{
			name: snap.Name,
			when: time.Unix(timestamp.Sec, timestamp.Nsec),
		}
	}
	return out, nil
}

func (i *CephImageView) SnapAndActivate(snapName string) error {
	_, err := i.image.CreateSnapshot(snapName)
	if err != nil {
		return util.WrapFmt(err, "error creating snapshot %s", snapName)
	}
	err = i.image.SetSnapshot(snapName)
	if err != nil {
		return util.WrapFmt(err, "error setting snapshot %s", snapName)
	}
	return nil
}

func (i *CephImageView) BlockSize() (uint64, error) {
	stat, err := i.image.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Obj_size, nil
}

func (i *CephImageView) DiffIter(snapName string, callback rbd.DiffIterateCallback) error {
	c := rbd.DiffIterateConfig{
		Offset: 0,
		// Length can be larger than needed
		Length:        (1 << 62) - 1,
		SnapName:      snapName,
		IncludeParent: rbd.IncludeParent,
		WholeObject:   rbd.DisableWholeObject,
		Callback:      callback,
	}
	err := i.image.DiffIterate(c)
	if err != nil {
		return err
	}
	return nil
}

func (i *CephImageView) Read(offset uint64, length uint64) ([]byte, error) {
	out := make([]byte, length)
	_, err := i.image.ReadAt(out, int64(offset))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func NewCephImageView(image *rbd.Image) *CephImageView {
	return &CephImageView{image: image}
}

func Connect(cfg *config.CephClusterConfig) (*rados.Conn, error) {
	//conn, err := rados.NewConnWithClusterAndUser(cfg.ClusterName, cfg.AuthName)
	conn, err := rados.NewConn()
	if err != nil {
		return nil, util.Wrap("error creating rados connection", err)
	}
	err = conn.ReadConfigFile(cfg.ConfFile)
	if err != nil {
		return nil, util.Wrap("error reading config file", err)
	}
	err = conn.Connect()
	if err != nil {
		return nil, util.Wrap("error connecting to ceph server", err)
	}
	return conn, nil
}
