package config

import (
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/zfssupport"
	"regexp"
)

const DEFAULT_MAX_CONC = 2

type TopLevelRawConfig struct {
	Clusters map[string]*CephClusterConfig `yaml:"clusters" binding:"required"`
	Jobs     []*RbdPoolJobRawConfig        `yaml:"jobs" binding:"required"`
}

type TopLevelProcessedConfig struct {
	Jobs    []*RbdPoolJobProcessedConfig
	Globals GlobalProcessedConfig
}

type GlobalProcessedConfig struct {
	DisableAllCron bool
}

type CephClusterConfig struct {
	AuthName    string `yaml:"authName" binding:"required"`
	ConfFile    string `yaml:"confFile" binding:"required"`
	ClusterName string `yaml:"clusterName" binding:"required"`
}

var DefaultClusterConfig = &CephClusterConfig{
	AuthName:    "client.admin",
	ConfFile:    "/etc/ceph/ceph.conf",
	ClusterName: "ceph",
}

type RbdPoolJobRawConfig struct {
	Id                string      `yaml:"id" binding:"required"`
	Label             string      `yaml:"label" binding:"required"`
	Cluster           string      `yaml:"cluster" binding:"required"`
	CephPoolName      string      `yaml:"cephPoolName" binding:"required"`
	ZfsDestination    string      `yaml:"zfsDestination" binding:"required"`
	ImageIncludeRegex string      `yaml:"imageIncludeRegex" binding:"required"`
	ImageExcludeRegex string      `yaml:"imageExcludeRegex" binding:"required"`
	MaxConcurrency    *int        `yaml:"maxConcurrency" binding:"required"`
	Pruning           *PruningRaw `yaml:"pruning"`
	Cron              *string     `yaml:"cron""`
}

type PruningRaw struct {
	KeepSender   []pruning.PruningEnum `yaml:"keepSender"`
	KeepReceiver []pruning.PruningEnum `yaml:"keepReceiver"`
}

type RbdPoolJobProcessedConfig struct {
	Id                string
	Label             string
	ClusterConfig     *CephClusterConfig
	CephPoolName      string
	ZfsDestination    string
	ImageIncludeRegex *regexp.Regexp
	ImageExcludeRegex *regexp.Regexp
	MaxConcurrency    int
	SrcPruning        pruning.Pruner[*models.CephSnapshot]
	RcvPruning        pruning.Pruner[*zfssupport.ZvolSnapshot]
	Cron              *string
}
