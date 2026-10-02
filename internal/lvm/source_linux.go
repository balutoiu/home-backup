// Package lvm provides a Linux LVM snapshot backup source.
package lvm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/balutoiu/home-backup/internal/backup"
	"github.com/balutoiu/home-backup/internal/command"
)

// CommandRunner executes the system commands needed by an LVM source.
type CommandRunner interface {
	Run(context.Context, command.Spec) (command.Result, error)
}

// Mounter mounts snapshot devices in directories it owns.
type Mounter interface {
	// Mount mounts device read-only in a new directory and returns its path.
	Mount(ctx context.Context, device string) (string, error)
	// Unmount unmounts path and removes the directory Mount created.
	Unmount(path string) error
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
	// Logger reports a stale snapshot removed by Open; it must not be nil.
	Logger *slog.Logger
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
	snap := snapshot{
		runner: s.deps.Runner,
		vgName: s.config.VGName,
		lvName: s.config.LVName,
		size:   s.config.SnapshotSize,
	}
	removed, err := snap.removeStale(ctx)
	if err != nil {
		return nil, err
	}
	if removed {
		s.deps.Logger.WarnContext(ctx, "removed stale LVM snapshot", "snapshot", snap.path())
	}
	if err := snap.create(ctx); err != nil {
		return nil, err
	}

	mountPath, err := s.deps.Mounter.Mount(ctx, snap.path())
	if err != nil {
		mountErr := fmt.Errorf("mount LVM snapshot %q: %w", snap.path(), err)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backup.ReleaseTimeout)
		defer cancel()
		if removeErr := snap.remove(cleanupCtx); removeErr != nil {
			return nil, errors.Join(mountErr, removeErr)
		}
		return nil, mountErr
	}
	return &input{mountPath: mountPath, snapshot: snap, mounter: s.deps.Mounter}, nil
}

// snapshot is the LVM snapshot of one logical volume, taken for one backup.
type snapshot struct {
	runner CommandRunner
	vgName string
	lvName string
	size   string
}

func (s snapshot) name() string { return s.lvName + "_backup_snapshot" }

func (s snapshot) path() string { return fmt.Sprintf("/dev/%s/%s", s.vgName, s.name()) }

func (s snapshot) create(ctx context.Context) error {
	_, err := s.runner.Run(ctx, command.Spec{
		Name: "lvcreate",
		Args: []string{
			"--snapshot",
			"--size", s.size,
			"--name", s.name(),
			fmt.Sprintf("/dev/%s/%s", s.vgName, s.lvName),
		},
	})
	if err != nil {
		return fmt.Errorf("create LVM snapshot %q: %w", s.path(), err)
	}
	return nil
}

func (s snapshot) remove(ctx context.Context) error {
	if err := s.lvremove(ctx); err != nil {
		return fmt.Errorf("remove LVM snapshot %q: %w", s.path(), err)
	}
	return nil
}

// removeStale removes a snapshot of this volume left behind by an earlier Run
// and reports whether there was one. It refuses to touch any other volume
// that has the snapshot's name.
func (s snapshot) removeStale(ctx context.Context) (bool, error) {
	result, err := s.runner.Run(ctx, command.Spec{
		Name: "lvs",
		Args: []string{"--noheadings", "--options", "origin", s.vgName + "/" + s.name()},
	})
	if err != nil {
		// Usually there is no such volume. Any other LVM problem fails lvcreate.
		return false, nil
	}
	if origin := strings.TrimSpace(result.Stdout); origin != s.lvName {
		return false, fmt.Errorf("LVM volume %q exists but is not a snapshot of %q", s.path(), s.lvName)
	}
	if err := s.lvremove(ctx); err != nil {
		return false, fmt.Errorf("remove stale LVM snapshot %q: %w", s.path(), err)
	}
	return true, nil
}

func (s snapshot) lvremove(ctx context.Context) error {
	_, err := s.runner.Run(ctx, command.Spec{
		Name: "lvremove",
		Args: []string{"--force", s.path()},
	})
	return err
}

type input struct {
	mountPath string
	snapshot  snapshot
	mounter   Mounter
}

func (i *input) Path() string { return i.mountPath }

// Release undoes Open in reverse: unmount, then remove the snapshot.
func (i *input) Release(ctx context.Context) error {
	if err := i.mounter.Unmount(i.mountPath); err != nil {
		// A mounted snapshot is open, so lvremove would fail too.
		return fmt.Errorf("unmount %q: %w", i.mountPath, err)
	}
	return i.snapshot.remove(ctx)
}

var _ backup.Source = (*Source)(nil)
var _ backup.Input = (*input)(nil)
