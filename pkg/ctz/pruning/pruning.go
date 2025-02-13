package pruning

import (
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type PruningEnum struct {
	Ret interface{}
}

//func (t *PruningEnum) UnmarshalYAML(value *yaml.Node) (err error) {
//	t.Ret, err = enumUnmarshal(value, map[string]interface{}{
//		// Is there a better/cleaner way?
//		"lastN": &PruneKeepLastN{},
//		"grid":  &PruneGrid{},
//		"regex": &PruneKeepRegex{},
//	})
//	return
//}
//
//var _ yaml.Unmarshaler = &PruningEnum{}

type PruneKeepLastN struct {
	Type  string `yaml:"type"`
	Count int    `yaml:"count"`
	Regex string `yaml:"regex"`
}

type PruneKeepRegex struct { // FIXME rename to KeepRegex
	Type   string `yaml:"type"`
	Regex  string `yaml:"regex"`
	Negate bool   `yaml:"negate,default=false"`
}

// TODO: this is implementing the "obsoleteUnmarshaler" interface
func (t *PruningEnum) UnmarshalYAML(u func(interface{}) error) (err error) {
	t.Ret, err = enumUnmarshalOld(u, map[string]interface{}{
		"lastN": &PruneKeepLastN{},
		"grid":  &PruneGrid{},
		"regex": &PruneKeepRegex{},
	})
	_ = len("")
	return
}

func enumUnmarshal(value *yaml.Node, types map[string]interface{}) (interface{}, error) {
	var in struct {
		Type string
	}
	err := value.Decode(&in)
	if err != nil {
		return nil, err
	}
	if in.Type == "" {
		return nil, &yaml.TypeError{Errors: []string{"must specify type"}}
	}
	v, ok := types[in.Type]
	if !ok {
		return nil, &yaml.TypeError{Errors: []string{fmt.Sprintf("invalid type name %q", in.Type)}}
	}
	// TODO: value.Decode is not an acceptable substitute, as it causes it to lose the type info
	// i.e. v goes from {interface{} | *Whatever} to {interface{} | map[string]interface{}}
	err = value.Decode(&v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func enumUnmarshalOld(u func(interface{}) error, types map[string]interface{}) (interface{}, error) {
	var in struct {
		Type string
	}
	if err := u(&in); err != nil {
		return nil, err
	}
	if in.Type == "" {
		return nil, &yaml.TypeError{Errors: []string{"must specify type"}}
	}

	v, ok := types[in.Type]
	if !ok {
		return nil, &yaml.TypeError{Errors: []string{fmt.Sprintf("invalid type name %q", in.Type)}}
	}
	if err := u(v); err != nil {
		return nil, err
	}
	return v, nil
}

// The returned snapshot list is guaranteed to only contains elements of input parameter snaps
func PruneSnapshots(snaps []models.Snapshot, keepRules []KeepRule) []models.Snapshot {

	if len(keepRules) == 0 {
		return []models.Snapshot{}
	}

	remCount := make(map[models.Snapshot]int, len(snaps))
	for _, r := range keepRules {
		ruleRems := r.KeepRule(snaps)
		for _, ruleRem := range ruleRems {
			remCount[ruleRem]++
		}
	}

	remove := make([]models.Snapshot, 0, len(snaps))
	for snap, rc := range remCount {
		if rc == len(keepRules) {
			remove = append(remove, snap)
		}
	}

	return remove
}

func RulesFromConfig(in []PruningEnum) (rules []KeepRule, err error) {
	rules = make([]KeepRule, len(in))
	for i := range in {
		rules[i], err = RuleFromConfig(in[i])
		if err != nil {
			return nil, errors.Wrapf(err, "cannot build rule #%d", i)
		}
	}
	return rules, nil
}

func RuleFromConfig(in PruningEnum) (KeepRule, error) {
	switch v := in.Ret.(type) {
	case *PruneKeepLastN:
		return NewKeepLastN(v.Count, v.Regex)
	case *PruneKeepRegex:
		return NewKeepRegex(v.Regex, v.Negate)
	case *PruneGrid:
		return NewKeepGrid(v)
	default:
		return nil, fmt.Errorf("unknown keep rule type %T", v)
	}
}

type KeepRule interface {
	KeepRule(snaps []models.Snapshot) (destroyList []models.Snapshot)
}

type Pruning interface {
	DestroySender(snapshots []models.Snapshot) []models.Snapshot
	DestroyReceiver(snapshots []models.Snapshot) []models.Snapshot
}

type pruner struct {
	srcRules []KeepRule
	rcvRules []KeepRule
}

var _ Pruning = &pruner{}

func (p *pruner) DestroySender(snapshots []models.Snapshot) []models.Snapshot {
	return PruneSnapshots(snapshots, p.srcRules)
}

func (p *pruner) DestroyReceiver(snapshots []models.Snapshot) []models.Snapshot {
	return PruneSnapshots(snapshots, p.rcvRules)
}

func NewPruner(srcRules []KeepRule, rcvRules []KeepRule) Pruning {
	return &pruner{srcRules: srcRules, rcvRules: rcvRules}
}

type noopPruner struct {
}

var _ Pruning = &noopPruner{}

func (n *noopPruner) DestroySender(snapshots []models.Snapshot) []models.Snapshot {
	return []models.Snapshot{}
}

func (n *noopPruner) DestroyReceiver(snapshots []models.Snapshot) []models.Snapshot {
	return []models.Snapshot{}
}

func NoPruner() Pruning {
	return &noopPruner{}
}
