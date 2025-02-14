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
func PruneSnapshots[T models.Snapshot](snaps []T, keepRules []KeepRule[T]) []T {

	if len(keepRules) == 0 {
		return []T{}
	}

	remCount := make(map[models.Snapshot]int, len(snaps))
	for _, r := range keepRules {
		ruleRems := r.KeepRule(snaps)
		for _, ruleRem := range ruleRems {
			remCount[ruleRem]++
		}
	}

	remove := make([]T, 0, len(snaps))
	for snap, rc := range remCount {
		if rc == len(keepRules) {
			remove = append(remove, snap.(T))
		}
	}

	return remove
}

func RulesFromConfig[T models.Snapshot](in []PruningEnum) (rules []KeepRule[T], err error) {
	rules = make([]KeepRule[T], len(in))
	for i := range in {
		rules[i], err = RuleFromConfig[T](in[i])
		if err != nil {
			return nil, errors.Wrapf(err, "cannot build rule #%d", i)
		}
	}
	return rules, nil
}

func RuleFromConfig[T models.Snapshot](in PruningEnum) (KeepRule[T], error) {
	switch v := in.Ret.(type) {
	case *PruneKeepLastN:
		return NewKeepLastN[T](v.Count, v.Regex)
	case *PruneKeepRegex:
		return NewKeepRegex[T](v.Regex, v.Negate)
	case *PruneGrid:
		return NewKeepGrid[T](v)
	default:
		return nil, fmt.Errorf("unknown keep rule type %T", v)
	}
}

type KeepRule[T models.Snapshot] interface {
	KeepRule(snaps []T) (destroyList []T)
}

type Pruner[T models.Snapshot] interface {
	Destroy(snapshots []T) []T
}

type pruner[T models.Snapshot] struct {
	rules []KeepRule[T]
}

func (p *pruner[T]) Destroy(snapshots []T) []T {
	return PruneSnapshots(snapshots, p.rules)
}

var _ Pruner[models.Snapshot] = &pruner[models.Snapshot]{}

func NewPruner[T models.Snapshot](rules []KeepRule[T]) Pruner[T] {
	return &pruner[T]{rules: rules}
}

type noopPruner[T models.Snapshot] struct {
}

func (n *noopPruner[T]) Destroy(snapshots []T) []T {
	return []T{}
}

var _ Pruner[models.Snapshot] = &noopPruner[models.Snapshot]{}

func NoPruner[T models.Snapshot]() Pruner[T] {
	return &noopPruner[T]{}
}
