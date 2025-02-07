package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

type TopLevelRawConfig struct {
	Clusters map[string]*CephClusterConfig `yaml:"clusters" binding:"required"`
	Jobs     []*PoolJobRawConfig           `yaml:"jobs" binding:"required"`
}

type TopLevelProcessedConfig struct {
	Jobs []*PoolJobProcessedConfig
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

type PoolJobRawConfig struct {
	Name              string `yaml:"name" binding:"required"`
	Cluster           string `yaml:"cluster" binding:"required"`
	CephPoolName      string `yaml:"cephPoolName" binding:"required"`
	ZfsDestination    string `yaml:"zfsDestination" binding:"required"`
	ImageIncludeRegex string `yaml:"imageIncludeRegex" binding:"required"`
	ImageExcludeRegex string `yaml:"imageExcludeRegex" binding:"required"`
}

type PoolJobProcessedConfig struct {
	Name              string
	ClusterConfig     *CephClusterConfig
	CephPoolName      string
	ZfsDestination    string
	ImageIncludeRegex *regexp.Regexp
	ImageExcludeRegex *regexp.Regexp
}

func FromYamlFile(filepath string) (*TopLevelProcessedConfig, error) {
	var rawConfig TopLevelRawConfig
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(bytes, &rawConfig)
	if err != nil {
		return nil, err
	}
	for _, cluster := range rawConfig.Clusters {
		if cluster.AuthName == "" {
			cluster.AuthName = DefaultClusterConfig.AuthName
		}
		if cluster.ConfFile == "" {
			cluster.ConfFile = DefaultClusterConfig.ConfFile
		}
		if cluster.ClusterName == "" {
			cluster.ClusterName = DefaultClusterConfig.ClusterName
		}
	}
	var jobs []*PoolJobProcessedConfig
	for i, rawJob := range rawConfig.Jobs {
		clusterKey := rawJob.Cluster
		clusterConfig := rawConfig.Clusters[clusterKey]
		if clusterConfig == nil {
			return nil, errors.New(fmt.Sprintf("Job '%v' wants cluster '%v', but there is no configured cluster of that name", rawJob.Name, clusterKey))
		}
		var include *regexp.Regexp
		if rawJob.ImageIncludeRegex != "" {
			include, err = regexp.Compile(rawJob.ImageIncludeRegex)
			if err != nil {
				return nil, err
			}
		}
		var exclude *regexp.Regexp
		if rawJob.ImageExcludeRegex != "" {
			exclude, err = regexp.Compile(rawJob.ImageExcludeRegex)
			if err != nil {
				return nil, err
			}
		}
		if rawJob.CephPoolName == "" {
			return nil, errors.New(fmt.Sprintf("name is missing in job config '%v'", i))
		}
		if rawJob.CephPoolName == "" {
			return nil, errors.New(fmt.Sprintf("cephPoolName is missing in job config '%v'", rawJob.Name))
		}
		if rawJob.ZfsDestination == "" {
			return nil, errors.New(fmt.Sprintf("zfsDestination is missing in job config '%v'", rawJob.Name))
		}
		job := &PoolJobProcessedConfig{
			Name:              rawJob.Name,
			ClusterConfig:     clusterConfig,
			CephPoolName:      rawJob.CephPoolName,
			ZfsDestination:    rawJob.ZfsDestination,
			ImageIncludeRegex: include,
			ImageExcludeRegex: exclude,
		}
		jobs = append(jobs, job)
	}
	config := &TopLevelProcessedConfig{
		Jobs: jobs,
	}
	return config, nil
}
