package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsConfigFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("backups:\n- source: {type: directory, path: /tmp}\n  destination: {type: restic, repo: /repo}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Backups) != 1 {
		t.Fatalf("len(Backups) = %d, want 1", len(cfg.Backups))
	}
}

func TestDecodeValidConfig(t *testing.T) {
	t.Parallel()

	yaml := `backups:
  - source:
      type: directory
      path: /srv/photos
    destination:
      type: restic
      repo: /backup/photos
  - source:
      type: lvm
      vg_name: vg0
      lv_name: home
    destination:
      type: restic
      repo: rclone:remote:home
      keep_last: "4"
      group_by: paths
`

	cfg, err := Decode(strings.NewReader(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(cfg.Backups) != 2 {
		t.Fatalf("len(Backups) = %d, want 2", len(cfg.Backups))
	}
	if got := cfg.Backups[0].Source; got.Kind != SourceDirectory || got.Directory == nil || got.Directory.Path != "/srv/photos" {
		t.Fatalf("directory source = %#v", got)
	}
	if got := cfg.Backups[0].Destination.Restic; got == nil || got.KeepLast != DefaultResticKeepLast || got.GroupBy != DefaultResticGroupBy {
		t.Fatalf("default Restic destination = %#v", got)
	}
	if got := cfg.Backups[1].Source; got.Kind != SourceLVM || got.LVM == nil || got.LVM.VGName != "vg0" || got.LVM.LVName != "home" {
		t.Fatalf("LVM source = %#v", got)
	}
	if got := cfg.Backups[1].Destination.Restic; got == nil || got.KeepLast != 4 || got.GroupBy != "paths" {
		t.Fatalf("explicit Restic destination = %#v", got)
	}
}

func TestDecodeAcceptsNumericKeepLast(t *testing.T) {
	t.Parallel()

	yaml := `backups:
  - source: {type: directory, path: /tmp}
    destination: {type: restic, repo: /repo, keep_last: 3}
`
	cfg, err := Decode(strings.NewReader(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got := cfg.Backups[0].Destination.Restic.KeepLast; got != 3 {
		t.Fatalf("KeepLast = %d, want 3", got)
	}
}

func TestDecodeRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "empty backups", yaml: "backups: []\n", want: "at least one backup"},
		{name: "unknown top-level field", yaml: "backups: []\nextra: true\n", want: "field extra not found"},
		{name: "unknown source field", yaml: "backups:\n- source: {type: directory, path: /tmp, typo: value}\n  destination: {type: restic, repo: /repo}\n", want: "unknown source field"},
		{name: "unknown source type", yaml: "backups:\n- source: {type: zfs}\n  destination: {type: restic, repo: /repo}\n", want: "unsupported source type"},
		{name: "missing directory path", yaml: "backups:\n- source: {type: directory}\n  destination: {type: restic, repo: /repo}\n", want: "directory source path is required"},
		{name: "negative retention", yaml: "backups:\n- source: {type: directory, path: /tmp}\n  destination: {type: restic, repo: /repo, keep_last: -1}\n", want: "keep_last cannot be negative"},
		{name: "multiple documents", yaml: "backups:\n- source: {type: directory, path: /tmp}\n  destination: {type: restic, repo: /repo}\n---\nbackups: []\n", want: "multiple YAML documents"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tt.yaml), "test.yaml")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Decode() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
