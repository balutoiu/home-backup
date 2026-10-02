package app

import (
	"context"
	"fmt"

	"github.com/balutoiu/home-backup/internal/backup"
	"github.com/balutoiu/home-backup/internal/command"
	"github.com/balutoiu/home-backup/internal/config"
	"github.com/balutoiu/home-backup/internal/directory"
	"github.com/balutoiu/home-backup/internal/lvm"
	"github.com/balutoiu/home-backup/internal/restic"
)

type commandRunner interface {
	Run(context.Context, command.Spec) (command.Result, error)
}

type wiringDependencies struct {
	runner commandRunner
	euid   func() int
}

func buildJobs(cfg config.Config, deps wiringDependencies) ([]backup.Job, error) {
	jobs := make([]backup.Job, 0, len(cfg.Backups))
	for i, spec := range cfg.Backups {
		source, err := buildSource(spec.Source, deps)
		if err != nil {
			return nil, fmt.Errorf("build backup %d source: %w", i+1, err)
		}
		destination, err := buildDestination(spec.Destination, deps)
		if err != nil {
			return nil, fmt.Errorf("build backup %d destination: %w", i+1, err)
		}
		jobs = append(jobs, backup.NewLocalJob(source, destination))
	}
	return jobs, nil
}

func buildSource(spec config.Source, deps wiringDependencies) (backup.Source, error) {
	switch spec.Kind {
	case config.SourceDirectory:
		return directory.NewSource(spec.Directory.Path), nil
	case config.SourceLVM:
		mounter := lvm.NewSystemMounter(deps.runner)
		return lvm.NewSource(lvm.Config{
			VGName: spec.LVM.VGName,
			LVName: spec.LVM.LVName,
		}, lvm.Dependencies{
			Runner:  deps.runner,
			Mounter: mounter,
			EUID:    deps.euid,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported source kind %q", spec.Kind)
	}
}

func buildDestination(spec config.Destination, deps wiringDependencies) (backup.Destination, error) {
	switch spec.Kind {
	case config.DestinationRestic:
		return restic.NewDestination(restic.Config{
			Repo:     spec.Restic.Repo,
			KeepLast: spec.Restic.KeepLast,
			GroupBy:  spec.Restic.GroupBy,
		}, deps.runner), nil
	default:
		return nil, fmt.Errorf("unsupported destination kind %q", spec.Kind)
	}
}
