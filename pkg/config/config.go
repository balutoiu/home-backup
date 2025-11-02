package config

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Backups []Backup `yaml:"backups"`
}

type Backup struct {
	Source      Source      `yaml:"source"`
	Destination Destination `yaml:"destination"`
}

type Source struct {
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
}

type Destination struct {
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
}

// LoadConfig reads and parses the YAML configuration file.
func LoadConfig(filePath string) (*Config, error) {
	log.Debugf("loading configuration from: %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	log.Debugf("unmarshalling configuration data from YAML")
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
