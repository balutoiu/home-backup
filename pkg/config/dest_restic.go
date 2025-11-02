package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type DestResticParams struct {
	Repo string `yaml:"repo"`
}

func (d *DestResticParams) ParseParams(params map[string]string) error {
	bytes, err := yaml.Marshal(params)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	return nil
}

func (d *DestResticParams) validate() error {
	if d.Repo == "" {
		return fmt.Errorf("restic destination 'repo' parameter is required")
	}
	return nil
}
