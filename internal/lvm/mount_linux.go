package lvm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/balutoiu/home-backup/internal/command"
	"golang.org/x/sys/unix"
)

// SystemMounter mounts snapshot devices through Linux mount syscalls, each in
// a temporary directory it creates on Mount and removes on Unmount.
type SystemMounter struct {
	runner CommandRunner
	// dir holds the mount directories; empty means os.TempDir.
	dir     string
	mount   func(source, target, fstype string, flags uintptr, data string) error
	unmount func(target string, flags int) error
}

// NewSystemMounter constructs a Linux system mounter.
func NewSystemMounter(runner CommandRunner) *SystemMounter {
	return &SystemMounter{runner: runner, mount: unix.Mount, unmount: unix.Unmount}
}

// Mount detects the filesystem and mounts device read-only in a new directory.
func (m *SystemMounter) Mount(ctx context.Context, device string) (string, error) {
	result, err := m.runner.Run(ctx, command.Spec{
		Name: "blkid",
		Args: []string{"-o", "value", "-s", "TYPE", device},
	})
	if err != nil {
		return "", fmt.Errorf("detect filesystem on %q: %w", device, err)
	}
	filesystem := strings.TrimSpace(result.Stdout)
	if filesystem == "" {
		return "", fmt.Errorf("no filesystem detected on %q", device)
	}

	mountPath, err := os.MkdirTemp(m.dir, "lvm-backup-*")
	if err != nil {
		return "", fmt.Errorf("create mount directory: %w", err)
	}
	data := ""
	if filesystem == "xfs" {
		// The snapshot shares its origin's UUID, and XFS refuses to mount a
		// duplicate while the origin is mounted.
		data = "nouuid"
	}
	if err := m.mount(device, mountPath, filesystem, unix.MS_RDONLY, data); err != nil {
		mountErr := fmt.Errorf("mount %q at %q: %w", device, mountPath, err)
		if removeErr := os.Remove(mountPath); removeErr != nil {
			return "", errors.Join(mountErr, fmt.Errorf("remove mount directory %q: %w", mountPath, removeErr))
		}
		return "", mountErr
	}
	return mountPath, nil
}

// Unmount unmounts path and removes its directory. A directory that is still
// mounted is left alone.
func (m *SystemMounter) Unmount(path string) error {
	if err := m.unmount(path, 0); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove mount directory %q: %w", path, err)
	}
	return nil
}

var _ Mounter = (*SystemMounter)(nil)
