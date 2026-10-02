package config

import (
	"errors"

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

func decodeRestic(node yaml.Node) (ResticDestination, error) {
	d := ResticDestination{KeepLast: DefaultResticKeepLast, GroupBy: DefaultResticGroupBy}
	if err := decodeStrict(node, "destination", &d); err != nil {
		return ResticDestination{}, err
	}
	if d.Repo == "" {
		return ResticDestination{}, errors.New("restic destination repo is required")
	}
	if d.KeepLast < 1 {
		return ResticDestination{}, errors.New("restic destination keep_last must be at least 1")
	}
	if d.GroupBy == "" {
		return ResticDestination{}, errors.New("restic destination group_by cannot be empty")
	}
	return d, nil
}
