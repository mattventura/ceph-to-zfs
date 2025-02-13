package builder

import (
	"errors"
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

var idPattern = regexp.MustCompile("^[a-zA-Z0-9._-]+$")

func yamlFileToRaw(path string) (*config.TopLevelRawConfig, error) {
	var rawConfig config.TopLevelRawConfig
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(bytes, &rawConfig)
	if err != nil {
		return nil, err
	}
	return &rawConfig, nil
}

func FromYamlFile(path string) (*config.TopLevelProcessedConfig, error) {
	rawConfig, err := yamlFileToRaw(path)
	if err != nil {
		return nil, err
	}
	for _, cluster := range rawConfig.Clusters {
		if cluster.AuthName == "" {
			cluster.AuthName = config.DefaultClusterConfig.AuthName
		}
		if cluster.ConfFile == "" {
			cluster.ConfFile = config.DefaultClusterConfig.ConfFile
		}
		if cluster.ClusterName == "" {
			cluster.ClusterName = config.DefaultClusterConfig.ClusterName
		}
	}
	var jobs []*config.PoolJobProcessedConfig
	jobIds := make(map[string]bool)
	for i, rawJob := range rawConfig.Jobs {
		clusterKey := rawJob.Cluster
		if jobIds[rawJob.Id] == true {
			return nil, errors.New("duplicate job name: " + rawJob.Id)
		}
		if rawJob.Id == "" {
			return nil, errors.New(fmt.Sprintf("job id must be specified (job #%v)", i))
		}
		if !idPattern.MatchString(rawJob.Id) {
			return nil, errors.New("job id must contain only alphanumeric, hyphen, underscores: '" + rawJob.Id + "'")
		}
		jobIds[rawJob.Id] = true
		if rawJob.Label == "" {
			rawJob.Label = rawJob.Id
		}
		clusterConfig := rawConfig.Clusters[clusterKey]
		if clusterConfig == nil {
			return nil, errors.New(fmt.Sprintf("Job '%v' wants cluster '%v', but there is no configured cluster of that name", rawJob.Label, clusterKey))
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
			return nil, errors.New(fmt.Sprintf("cephPoolName is missing in job config '%v'", rawJob.Label))
		}
		if rawJob.ZfsDestination == "" {
			return nil, errors.New(fmt.Sprintf("zfsDestination is missing in job config '%v'", rawJob.Label))
		}
		var conc int
		if rawJob.MaxConcurrency != nil {
			conc = *rawJob.MaxConcurrency
			if conc < 1 {
				return nil, errors.New(fmt.Sprintf("maxConcurrency '%v' is invalid - must be greater than 0", rawJob.MaxConcurrency))
			}
		} else {
			conc = config.DEFAULT_MAX_CONC
		}

		var prune pruning.Pruning
		if rawJob.Pruning != nil {
			sender, err := pruning.RulesFromConfig(rawJob.Pruning.KeepSender)
			if err != nil {
				return nil, err
			}
			receiver, err := pruning.RulesFromConfig(rawJob.Pruning.KeepReceiver)
			if err != nil {
				return nil, err
			}
			prune = pruning.NewPruner(sender, receiver)
		} else {
			prune = pruning.NoPruner()
		}
		job := &config.PoolJobProcessedConfig{
			Id:                rawJob.Id,
			Label:             rawJob.Label,
			ClusterConfig:     clusterConfig,
			CephPoolName:      rawJob.CephPoolName,
			ZfsDestination:    rawJob.ZfsDestination,
			ImageIncludeRegex: include,
			ImageExcludeRegex: exclude,
			MaxConcurrency:    conc,
			Pruning:           prune,
		}
		jobs = append(jobs, job)
	}
	cfg := &config.TopLevelProcessedConfig{
		Jobs: jobs,
	}
	return cfg, nil
}
