package config

import (
	"errors"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

const resticType = "restic"

const (
	// DefaultResticKeepLast is the default number of snapshots retained.
	DefaultResticKeepLast = 10
	// DefaultResticGroupBy is the default Restic snapshot grouping.
	DefaultResticGroupBy = "host"
)

// ResticDestination configures a Restic destination.
type ResticDestination struct {
	Repo     string `yaml:"repo"`
	KeepLast int    `yaml:"keep_last"`
	GroupBy  string `yaml:"group_by"`
}

func (ResticDestination) isDestination() {}

type rawRestic struct {
	Repo     string      `yaml:"repo"`
	KeepLast optionalInt `yaml:"keep_last"`
	GroupBy  string      `yaml:"group_by"`
}

type optionalInt struct {
	value int
	set   bool
}

func (v *optionalInt) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected integer scalar at line %d", node.Line)
	}
	value, err := strconv.Atoi(node.Value)
	if err != nil {
		return fmt.Errorf("parse integer %q at line %d: %w", node.Value, node.Line, err)
	}
	v.value = value
	v.set = true
	return nil
}

func decodeRestic(node yaml.Node) (ResticDestination, error) {
	var raw rawRestic
	if err := decodeStrict(node, "destination", &raw); err != nil {
		return ResticDestination{}, err
	}
	d := ResticDestination{Repo: raw.Repo, KeepLast: DefaultResticKeepLast, GroupBy: raw.GroupBy}
	if raw.KeepLast.set && raw.KeepLast.value != 0 {
		d.KeepLast = raw.KeepLast.value
	}
	if d.GroupBy == "" {
		d.GroupBy = DefaultResticGroupBy
	}
	if d.Repo == "" {
		return ResticDestination{}, errors.New("restic destination repo is required")
	}
	if d.KeepLast < 0 {
		return ResticDestination{}, errors.New("restic destination keep_last cannot be negative")
	}
	return d, nil
}
