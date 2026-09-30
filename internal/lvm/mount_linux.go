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

// SystemMounter mounts snapshot devices through Linux mount syscalls.
type SystemMounter struct {
	runner CommandRunner
}

// NewSystemMounter constructs a Linux system mounter.
func NewSystemMounter(runner CommandRunner) *SystemMounter {
	return &SystemMounter{runner: runner}
}

// Mount detects the filesystem and mounts device read-only in a temporary directory.
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

	mountPath, err := os.MkdirTemp("", "lvm-backup-*")
	if err != nil {
		return "", fmt.Errorf("create mount directory: %w", err)
	}
	data := ""
	if filesystem == "xfs" {
		data = "nouuid"
	}
	if err := unix.Mount(device, mountPath, filesystem, unix.MS_RDONLY, data); err != nil {
		mountErr := fmt.Errorf("mount %q at %q: %w", device, mountPath, err)
		if removeErr := os.RemoveAll(mountPath); removeErr != nil {
			return "", errors.Join(mountErr, fmt.Errorf("remove mount directory %q: %w", mountPath, removeErr))
		}
		return "", mountErr
	}
	return mountPath, nil
}

// Unmount unmounts a snapshot path.
func (m *SystemMounter) Unmount(path string) error {
	return unix.Unmount(path, 0)
}

var _ Mounter = (*SystemMounter)(nil)
