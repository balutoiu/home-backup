package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type rawConfig struct {
	Backups []rawBackup `yaml:"backups"`
}

type rawBackup struct {
	Source      yaml.Node `yaml:"source"`
	Destination yaml.Node `yaml:"destination"`
}

// Load reads, decodes, and validates a configuration file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return Decode(bytes.NewReader(data), path)
}

// Decode decodes and validates one YAML configuration document.
func Decode(r io.Reader, source string) (Config, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	var raw rawConfig
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("decode config %q: multiple YAML documents are not supported", source)
		}
		return Config{}, fmt.Errorf("decode config %q: %w", source, err)
	}
	if len(raw.Backups) == 0 {
		return Config{}, fmt.Errorf("validate config %q: configuration requires at least one backup", source)
	}

	cfg := Config{Backups: make([]Backup, 0, len(raw.Backups))}
	for i, rawBackup := range raw.Backups {
		sourceSpec, err := decodeSource(rawBackup.Source)
		if err != nil {
			return Config{}, fmt.Errorf("decode config %q backup %d source: %w", source, i+1, err)
		}
		destinationSpec, err := decodeDestination(rawBackup.Destination)
		if err != nil {
			return Config{}, fmt.Errorf("decode config %q backup %d destination: %w", source, i+1, err)
		}
		cfg.Backups = append(cfg.Backups, Backup{Source: sourceSpec, Destination: destinationSpec})
	}
	return cfg, nil
}

func decodeSource(node yaml.Node) (Source, error) {
	kind, err := decodeType(node, "source")
	if err != nil {
		return nil, err
	}
	switch kind {
	case directoryType:
		return decodeDirectory(node)
	case lvmType:
		return decodeLVM(node)
	default:
		return nil, fmt.Errorf("unsupported source type %q (want %s or %s)", kind, directoryType, lvmType)
	}
}

func decodeDestination(node yaml.Node) (Destination, error) {
	kind, err := decodeType(node, "destination")
	if err != nil {
		return nil, err
	}
	switch kind {
	case resticType:
		return decodeRestic(node)
	default:
		return nil, fmt.Errorf("unsupported destination type %q (want %s)", kind, resticType)
	}
}

// decodeType returns the type field of a source or destination mapping.
func decodeType(node yaml.Node, subject string) (string, error) {
	if node.Kind != yaml.MappingNode {
		return "", fmt.Errorf("%s must be a mapping", subject)
	}
	var discriminator struct {
		Type string `yaml:"type"`
	}
	if err := node.Decode(&discriminator); err != nil {
		return "", fmt.Errorf("decode %s: %w", subject, err)
	}
	return discriminator.Type, nil
}

// decodeStrict decodes a mapping into target, a struct pointer, rejecting
// keys other than type and target's yaml tags. Node.Decode ignores
// KnownFields, so the check is done here.
func decodeStrict(node yaml.Node, subject string, target any) error {
	known := yamlKeys(reflect.TypeOf(target).Elem())
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Value != "type" && !known[key.Value] {
			return fmt.Errorf("unknown %s field %q at line %d", subject, key.Value, key.Line)
		}
	}
	if err := node.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", subject, err)
	}
	return nil
}

func yamlKeys(t reflect.Type) map[string]bool {
	keys := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			keys[name] = true
		}
	}
	return keys
}
