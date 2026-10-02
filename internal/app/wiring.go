package app

import (
	"context"
	"fmt"
	"log/slog"

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
	logger *slog.Logger
}

func buildBackups(cfg config.Config, deps wiringDependencies) ([]backup.Backup, error) {
	backups := make([]backup.Backup, 0, len(cfg.Backups))
	for i, spec := range cfg.Backups {
		source, err := buildSource(spec.Source, deps)
		if err != nil {
			return nil, fmt.Errorf("build backup %d source: %w", i+1, err)
		}
		destination, err := buildDestination(spec.Destination, deps)
		if err != nil {
			return nil, fmt.Errorf("build backup %d destination: %w", i+1, err)
		}
		backups = append(backups, backup.Backup{
			Label:       spec.Source.String(),
			Source:      source,
			Destination: destination,
		})
	}
	return backups, nil
}

func buildSource(spec config.Source, deps wiringDependencies) (backup.Source, error) {
	switch spec := spec.(type) {
	case config.DirectorySource:
		return directory.NewSource(spec.Path), nil
	case config.LVMSource:
		mounter := lvm.NewSystemMounter(deps.runner)
		return lvm.NewSource(lvm.Config{
			VGName:       spec.VGName,
			LVName:       spec.LVName,
			SnapshotSize: spec.SnapshotSize,
		}, lvm.Dependencies{
			Runner:  deps.runner,
			Mounter: mounter,
			EUID:    deps.euid,
			Logger:  deps.logger,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported source %T", spec)
	}
}

func buildDestination(spec config.Destination, deps wiringDependencies) (backup.Destination, error) {
	switch spec := spec.(type) {
	case config.ResticDestination:
		return restic.NewDestination(restic.Config{
			Repo:     spec.Repo,
			KeepLast: spec.KeepLast,
			GroupBy:  spec.GroupBy,
		}, deps.runner), nil
	default:
		return nil, fmt.Errorf("unsupported destination %T", spec)
	}
}
