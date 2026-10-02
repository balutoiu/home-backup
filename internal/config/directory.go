package config

import (
	"errors"

	"gopkg.in/yaml.v3"
)

const directoryType = "directory"

// DirectorySource configures a directory source.
type DirectorySource struct {
	Path string `yaml:"path"`
}

func (DirectorySource) isSource() {}

// String describes the source, e.g. "directory /srv/photos".
func (s DirectorySource) String() string { return directoryType + " " + s.Path }

func decodeDirectory(node yaml.Node) (DirectorySource, error) {
	var s DirectorySource
	if err := decodeStrict(node, "source", &s); err != nil {
		return DirectorySource{}, err
	}
	if s.Path == "" {
		return DirectorySource{}, errors.New("directory source path is required")
	}
	return s, nil
}
