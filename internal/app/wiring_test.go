package app

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/balutoiu/home-backup/internal/command"
	"github.com/balutoiu/home-backup/internal/config"
)

// fakeRunner succeeds with empty output, except for commands named in errs.
type fakeRunner struct {
	specs []command.Spec
	errs  map[string]error
}

func (f *fakeRunner) Run(_ context.Context, spec command.Spec) (command.Result, error) {
	f.specs = append(f.specs, spec)
	return command.Result{}, f.errs[spec.Name]
}

func TestBuildBackups(t *testing.T) {
	cfg := config.Config{Backups: []config.Backup{{
		Source:      config.DirectorySource{Path: "/srv/home"},
		Destination: config.ResticDestination{Repo: "/backups/restic", KeepLast: 5, GroupBy: "host"},
	}}}

	backups, err := buildBackups(cfg, wiringDependencies{
		runner: &fakeRunner{},
		euid:   func() int { return 0 },
	})
	if err != nil {
		t.Fatalf("buildBackups() error = %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("len(backups) = %d, want 1", len(backups))
	}
	if got := backups[0].Label; got != "directory /srv/home" {
		t.Fatalf("Label = %q, want %q", got, "directory /srv/home")
	}
}

func TestBuildBackupsPassesLVMSnapshotSize(t *testing.T) {
	// A failing lvs means there is no stale snapshot.
	runner := &fakeRunner{errs: map[string]error{"lvs": errors.New("not found")}}
	backups, err := buildBackups(config.Config{Backups: []config.Backup{{
		Source:      config.LVMSource{VGName: "vg0", LVName: "home", SnapshotSize: "2G"},
		Destination: config.ResticDestination{Repo: "/backups/restic", KeepLast: 5, GroupBy: "host"},
	}}}, wiringDependencies{runner: runner, euid: func() int { return 0 }, logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("buildBackups() error = %v", err)
	}

	// The fake blkid prints nothing, so Open stops before mounting.
	if _, err := backups[0].Source.Open(context.Background()); err == nil {
		t.Fatal("Open() error = nil, want missing filesystem")
	}
	for _, spec := range runner.specs {
		if spec.Name == "lvcreate" {
			if !slices.Contains(spec.Args, "2G") {
				t.Fatalf("lvcreate args = %v, want size 2G", spec.Args)
			}
			return
		}
	}
	t.Fatalf("commands = %v, want lvcreate", runner.specs)
}

func TestBuildBackupsRejectsMissingVariants(t *testing.T) {
	validSource := config.DirectorySource{Path: "/srv/home"}
	validDestination := config.ResticDestination{Repo: "/backups/restic", KeepLast: 5, GroupBy: "host"}
	tests := []struct {
		name        string
		backup      config.Backup
		wantMessage string
	}{
		{
			name:        "source",
			backup:      config.Backup{Destination: validDestination},
			wantMessage: "unsupported source",
		},
		{
			name:        "destination",
			backup:      config.Backup{Source: validSource},
			wantMessage: "unsupported destination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildBackups(config.Config{Backups: []config.Backup{tt.backup}}, wiringDependencies{
				runner: &fakeRunner{},
				euid:   func() int { return 0 },
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("buildBackups() error = %v, want substring %q", err, tt.wantMessage)
			}
		})
	}
}
