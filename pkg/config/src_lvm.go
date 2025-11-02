package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type SrcLVMParams struct {
	VGName string `yaml:"vg_name"`
	LVName string `yaml:"lv_name"`
}

func (s *SrcLVMParams) ParseParams(params map[string]string) error {
	bytes, err := yaml.Marshal(params)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, s); err != nil {
		return err
	}
	if err := s.validate(); err != nil {
		return err
	}
	return nil
}

func (s *SrcLVMParams) validate() error {
	if s.VGName == "" {
		return fmt.Errorf("lvm source 'vg_name' parameter is required for LVM source")
	}
	if s.LVName == "" {
		return fmt.Errorf("lvm source 'lv_name' parameter is required for LVM source")
	}
	return nil
}
