package builder

import (
	"errors"
	"fmt"
	"github.com/adhocore/gronx"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/zfssupport"
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
	var jobs []*config.RbdPoolJobProcessedConfig
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

		var srcPrune pruning.Pruner[*models.CephSnapshot]
		var rcvPrune pruning.Pruner[*zfssupport.ZvolSnapshot]
		if rawJob.Pruning != nil {
			srcRules, err := pruning.RulesFromConfig[*models.CephSnapshot](rawJob.Pruning.KeepSender)
			if err != nil {
				return nil, err
			}
			srcPrune = pruning.NewPruner[*models.CephSnapshot](srcRules)
			rcvRules, err := pruning.RulesFromConfig[*zfssupport.ZvolSnapshot](rawJob.Pruning.KeepReceiver)
			if err != nil {
				return nil, err
			}
			rcvPrune = pruning.NewPruner[*zfssupport.ZvolSnapshot](rcvRules)

		} else {
			srcPrune = pruning.NoPruner[*models.CephSnapshot]()
			rcvPrune = pruning.NoPruner[*zfssupport.ZvolSnapshot]()
		}
		if rawJob.Cron != nil {
			valid := gronx.IsValid(*rawJob.Cron)
			if !valid {
				return nil, errors.New(fmt.Sprintf("cron is invalid (%v)", rawJob.Cron))
			}
		}
		job := &config.RbdPoolJobProcessedConfig{
			Id:                rawJob.Id,
			Label:             rawJob.Label,
			ClusterConfig:     clusterConfig,
			CephPoolName:      rawJob.CephPoolName,
			ZfsDestination:    rawJob.ZfsDestination,
			ImageIncludeRegex: include,
			ImageExcludeRegex: exclude,
			MaxConcurrency:    conc,
			SrcPruning:        srcPrune,
			RcvPruning:        rcvPrune,
			Cron:              rawJob.Cron,
		}
		jobs = append(jobs, job)
	}
	cfg := &config.TopLevelProcessedConfig{
		Jobs: jobs,
	}
	return cfg, nil
}
