package config

import (
	"errors"

	"gopkg.in/yaml.v3"
)

const lvmType = "lvm"

// DefaultLVMSnapshotSize is the LVM snapshot size used when none is configured.
const DefaultLVMSnapshotSize = "10G"

// LVMSource configures an LVM source.
type LVMSource struct {
	VGName       string `yaml:"vg_name"`
	LVName       string `yaml:"lv_name"`
	SnapshotSize string `yaml:"snapshot_size"`
}

func (LVMSource) isSource() {}

// String describes the source, e.g. "lvm vg0/home".
func (s LVMSource) String() string { return lvmType + " " + s.VGName + "/" + s.LVName }

func decodeLVM(node yaml.Node) (LVMSource, error) {
	s := LVMSource{SnapshotSize: DefaultLVMSnapshotSize}
	if err := decodeStrict(node, "source", &s); err != nil {
		return LVMSource{}, err
	}
	if s.VGName == "" {
		return LVMSource{}, errors.New("LVM source vg_name is required")
	}
	if s.LVName == "" {
		return LVMSource{}, errors.New("LVM source lv_name is required")
	}
	if s.SnapshotSize == "" {
		return LVMSource{}, errors.New("LVM source snapshot_size cannot be empty")
	}
	return s, nil
}
