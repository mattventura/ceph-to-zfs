package cephsupport

import (
	"ceph-to-zfs/pkg/ctz/config"
	"ceph-to-zfs/pkg/ctz/util"
	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

type CephImageView struct {
	image *rbd.Image
}

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

func (i *CephImageView) SnapAndActivate(snapName string) error {
	_, err := i.image.CreateSnapshot(snapName)
	if err != nil {
		return err
	}
	err = i.image.SetSnapshot(snapName)
	if err != nil {
		return err
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
	config := rbd.DiffIterateConfig{
		Offset: 0,
		// Length can be larger than needed
		Length:        (1 << 62) - 1,
		SnapName:      snapName,
		IncludeParent: rbd.IncludeParent,
		WholeObject:   rbd.DisableWholeObject,
		Callback:      callback,
	}
	err := i.image.DiffIterate(config)
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
		return nil, err
	}
	err = conn.ReadConfigFile(cfg.ConfFile)
	if err != nil {
		return nil, err
	}
	err = conn.Connect()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
