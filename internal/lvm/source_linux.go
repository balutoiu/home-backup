// Package lvm provides a Linux LVM snapshot backup source.
package lvm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/balutoiu/home-backup/internal/backup"
	"github.com/balutoiu/home-backup/internal/command"
)

// CommandRunner executes the system commands needed by an LVM source.
type CommandRunner interface {
	Run(context.Context, command.Spec) (command.Result, error)
}

// Mounter mounts and unmounts snapshot devices.
type Mounter interface {
	Mount(context.Context, string) (string, error)
	Unmount(string) error
}

// Config identifies an LVM logical volume and snapshot size.
type Config struct {
	VGName       string
	LVName       string
	SnapshotSize string
}

// Dependencies supplies the system boundaries used by Source.
type Dependencies struct {
	Runner  CommandRunner
	Mounter Mounter
	EUID    func() int
}

// Source opens an LVM snapshot as backup input.
type Source struct {
	config Config
	deps   Dependencies
}

// NewSource constructs an LVM source.
func NewSource(cfg Config, deps Dependencies) *Source {
	return &Source{config: cfg, deps: deps}
}

// Open creates and mounts a read-only snapshot.
func (s *Source) Open(ctx context.Context) (backup.Input, error) {
	if s.deps.EUID == nil || s.deps.EUID() != 0 {
		return nil, errors.New("LVM source requires root privileges")
	}
	if _, err := s.deps.Runner.Run(ctx, command.Spec{Name: "sync"}); err != nil {
		return nil, fmt.Errorf("sync filesystems: %w", err)
	}
	_, err := s.deps.Runner.Run(ctx, command.Spec{
		Name: "lvcreate",
		Args: []string{
			"--snapshot",
			"--size", s.config.SnapshotSize,
			"--name", s.snapshotName(),
			s.lvPath(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create LVM snapshot %q: %w", s.snapshotPath(), err)
	}

	mountPath, mountErr := s.deps.Mounter.Mount(ctx, s.snapshotPath())
	if mountErr != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		_, rollbackErr := s.deps.Runner.Run(cleanupCtx, command.Spec{
			Name: "lvremove",
			Args: []string{"--force", s.snapshotPath()},
		})
		if rollbackErr != nil {
			return nil, errors.Join(
				fmt.Errorf("mount LVM snapshot %q: %w", s.snapshotPath(), mountErr),
				fmt.Errorf("rollback LVM snapshot %q: %w", s.snapshotPath(), rollbackErr),
			)
		}
		return nil, fmt.Errorf("mount LVM snapshot %q: %w", s.snapshotPath(), mountErr)
	}

	return &input{
		mountPath:    mountPath,
		snapshotPath: s.snapshotPath(),
		runner:       s.deps.Runner,
		mounter:      s.deps.Mounter,
	}, nil
}

func (s *Source) lvPath() string {
	return fmt.Sprintf("/dev/%s/%s", s.config.VGName, s.config.LVName)
}

func (s *Source) snapshotName() string {
	return s.config.LVName + "_backup_snapshot"
}

func (s *Source) snapshotPath() string {
	return fmt.Sprintf("/dev/%s/%s", s.config.VGName, s.snapshotName())
}

type input struct {
	mountPath    string
	snapshotPath string
	runner       CommandRunner
	mounter      Mounter
}

func (i *input) Path() string { return i.mountPath }

func (i *input) Release(ctx context.Context) error {
	var errs []error
	if err := i.mounter.Unmount(i.mountPath); err != nil {
		errs = append(errs, fmt.Errorf("unmount %q: %w", i.mountPath, err))
	}
	_, err := i.runner.Run(ctx, command.Spec{
		Name: "lvremove",
		Args: []string{"--force", i.snapshotPath},
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("remove LVM snapshot %q: %w", i.snapshotPath, err))
	}
	if err := os.RemoveAll(i.mountPath); err != nil {
		errs = append(errs, fmt.Errorf("remove mount directory %q: %w", i.mountPath, err))
	}
	return errors.Join(errs...)
}

var _ backup.Source = (*Source)(nil)
var _ backup.Input = (*input)(nil)
