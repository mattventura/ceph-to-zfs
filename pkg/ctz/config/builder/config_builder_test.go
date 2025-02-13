package builder

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func TestYamlFileGood(t *testing.T) {
	cfg, err := FromYamlFile("../testdata/test.good.yaml")
	require.NoErrorf(t, err, "Error reading from yaml file")
	jobs := cfg.Jobs
	require.Len(t, jobs, 4)
	assert.Equal(t, &config.PoolJobProcessedConfig{
		Id:    "Backup_VMs",
		Label: "Backup VM Images",
		ClusterConfig: &config.CephClusterConfig{
			AuthName:    "client.admin",
			ConfFile:    "/etc/ceph/ceph.conf",
			ClusterName: "ceph",
		},
		CephPoolName:      "vm-pool",
		ZfsDestination:    "tank3/ceph-rbd-backups",
		ImageIncludeRegex: regexp.MustCompile("vm-\\d+-disk-.*"),
		ImageExcludeRegex: nil,
		MaxConcurrency:    3,
		Pruning:           pruning.NoPruner(),
	}, jobs[0])
	assert.Equal(t, &config.PoolJobProcessedConfig{
		Id:    "Backup_Templates",
		Label: "Backup VM Images 2 this job has a very long name",
		ClusterConfig: &config.CephClusterConfig{
			AuthName:    "client.backups",
			ConfFile:    "/etc/ceph/ceph2.conf",
			ClusterName: "ceph2",
		},
		CephPoolName:      "vm-pool",
		ZfsDestination:    "tank3/ceph-rbd-backups",
		ImageIncludeRegex: regexp.MustCompile("base-\\d+-disk-.*"),
		ImageExcludeRegex: nil,
		MaxConcurrency:    config.DEFAULT_MAX_CONC,
		Pruning:           pruning.NoPruner(),
	}, jobs[1])
	assert.Equal(t, &config.PoolJobProcessedConfig{
		Id:    "Empty",
		Label: "Dummy empty job",
		ClusterConfig: &config.CephClusterConfig{
			AuthName:    "client.admin",
			ConfFile:    "/etc/ceph/ceph.conf",
			ClusterName: "ceph",
		},
		CephPoolName:      "vm-pool",
		ZfsDestination:    "tank3/ceph-rbd-backups",
		ImageIncludeRegex: nil,
		ImageExcludeRegex: regexp.MustCompile("nothing"),
		MaxConcurrency:    config.DEFAULT_MAX_CONC,
		Pruning:           pruning.NoPruner(),
	}, jobs[2])
	assert.Equal(t, &config.PoolJobProcessedConfig{
		Id:    "Fails",
		Label: "Fails on purpose",
		ClusterConfig: &config.CephClusterConfig{
			AuthName:    "client.admin",
			ConfFile:    "/etc/ceph/ceph.conf",
			ClusterName: "ceph",
		},
		CephPoolName:      "nonexistent",
		ZfsDestination:    "tank3/ceph-rbd-backups",
		ImageIncludeRegex: regexp.MustCompile("foo"),
		ImageExcludeRegex: regexp.MustCompile("bar"),
		MaxConcurrency:    config.DEFAULT_MAX_CONC,
		Pruning:           pruning.NoPruner(),
	}, jobs[3])

	//assert.Equal(t, "Backup_VMs", jobs[0].Id)
	//assert.Equal(t, "myCluster", jobs[0].Label)

}

func TestYamlFilePruneRaw(t *testing.T) {
	cfg, err := yamlFileToRaw("../testdata/test.pruning.yaml")
	require.NoErrorf(t, err, "Error reading from yaml file")
	jobs := cfg.Jobs
	require.Len(t, jobs, 5)
	require.Nil(t, jobs[0].Pruning)
	require.Nil(t, jobs[1].Pruning.KeepSender)
	require.Nil(t, jobs[1].Pruning.KeepReceiver)

	require.Len(t, jobs[2].Pruning.KeepSender, 2)
	require.IsType(t, pruning.PruningEnum{}, jobs[2].Pruning.KeepSender[0])
	require.Nil(t, jobs[2].Pruning.KeepReceiver)
}
func TestYamlFilePrune(t *testing.T) {
	cfg, err := FromYamlFile("../testdata/test.pruning.yaml")
	require.NoErrorf(t, err, "Error reading from yaml file")
	// TODO
	_ = cfg
	//jobs := cfg.Jobs
	//require.Len(t, jobs, 5)
	//require.Nil(t, jobs[0].Pruning)
	//require.Nil(t, jobs[1].Pruning.KeepSender)
	//require.Nil(t, jobs[1].Pruning.KeepReceiver)
	//
	//require.Len(t, jobs[2].Pruning.KeepSender, 2)
	//require.IsType(t, pruning.PruningEnum{}, jobs[2].Pruning.KeepSender[0])
	//require.Nil(t, jobs[2].Pruning.KeepReceiver)
}
