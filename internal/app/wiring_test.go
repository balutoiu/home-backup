package app

import (
	"context"
	"strings"
	"testing"

	"github.com/balutoiu/home-backup/internal/command"
	"github.com/balutoiu/home-backup/internal/config"
)

type fakeRunner struct {
	specs []command.Spec
}

func (f *fakeRunner) Run(_ context.Context, spec command.Spec) (command.Result, error) {
	f.specs = append(f.specs, spec)
	return command.Result{ExitCode: 0}, nil
}

func TestBuildJobs(t *testing.T) {
	cfg := config.Config{Backups: []config.Backup{{
		Source: config.Source{
			Kind:      config.SourceDirectory,
			Directory: &config.DirectorySource{Path: "/srv/home"},
		},
		Destination: config.Destination{
			Kind: config.DestinationRestic,
			Restic: &config.ResticDestination{
				Repo:     "/backups/restic",
				KeepLast: 5,
				GroupBy:  "host",
			},
		},
	}}}

	jobs, err := buildJobs(cfg, wiringDependencies{
		runner: &fakeRunner{},
		euid:   func() int { return 0 },
	})
	if err != nil {
		t.Fatalf("buildJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
}

func TestBuildJobsRejectsUnsupportedKinds(t *testing.T) {
	validSource := config.Source{
		Kind:      config.SourceDirectory,
		Directory: &config.DirectorySource{Path: "/srv/home"},
	}
	validDestination := config.Destination{
		Kind: config.DestinationRestic,
		Restic: &config.ResticDestination{
			Repo:     "/backups/restic",
			KeepLast: 5,
			GroupBy:  "host",
		},
	}
	tests := []struct {
		name        string
		backup      config.Backup
		wantMessage string
	}{
		{
			name: "source",
			backup: config.Backup{
				Source:      config.Source{Kind: config.SourceKind("future-source")},
				Destination: validDestination,
			},
			wantMessage: "unsupported source kind",
		},
		{
			name: "destination",
			backup: config.Backup{
				Source:      validSource,
				Destination: config.Destination{Kind: config.DestinationKind("future-destination")},
			},
			wantMessage: "unsupported destination kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildJobs(config.Config{Backups: []config.Backup{tt.backup}}, wiringDependencies{
				runner: &fakeRunner{},
				euid:   func() int { return 0 },
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("buildJobs() error = %v, want substring %q", err, tt.wantMessage)
			}
		})
	}
}
