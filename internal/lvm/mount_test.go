//go:build linux

package lvm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/balutoiu/home-backup/internal/command"
	"golang.org/x/sys/unix"
)

type mountCall struct {
	source  string
	target  string
	fstype  string
	flags   uintptr
	data    string
	existed bool
}

// newTestMounter returns a SystemMounter whose syscalls are recorded and whose
// mount directories live in a test directory.
func newTestMounter(t *testing.T, runner CommandRunner, mountErr, unmountErr error) (*SystemMounter, *[]mountCall, *[]string) {
	t.Helper()
	var mounts []mountCall
	var unmounts []string
	m := &SystemMounter{
		runner: runner,
		dir:    t.TempDir(),
		mount: func(source, target, fstype string, flags uintptr, data string) error {
			_, err := os.Stat(target)
			mounts = append(mounts, mountCall{source, target, fstype, flags, data, err == nil})
			return mountErr
		},
		unmount: func(target string, _ int) error {
			unmounts = append(unmounts, target)
			return unmountErr
		},
	}
	return m, &mounts, &unmounts
}

func blkidRunner(stdout string, err error) *fakeRunner {
	return &fakeRunner{results: []command.Result{{Stdout: stdout}}, errs: []error{err}}
}

func mountDirs(t *testing.T, m *SystemMounter) []string {
	t.Helper()
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSystemMounterMountsDetectedFilesystemReadOnly(t *testing.T) {
	runner := blkidRunner("ext4\n", nil)
	m, mounts, _ := newTestMounter(t, runner, nil, nil)

	path, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	wantSpecs := []command.Spec{{
		Name: "blkid",
		Args: []string{"-o", "value", "-s", "TYPE", "/dev/vg0/home_backup_snapshot"},
	}}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
	if filepath.Dir(path) != m.dir || !strings.HasPrefix(filepath.Base(path), "lvm-backup-") {
		t.Fatalf("Mount() path = %q, want lvm-backup-* in %q", path, m.dir)
	}
	want := []mountCall{{"/dev/vg0/home_backup_snapshot", path, "ext4", unix.MS_RDONLY, "", true}}
	if !reflect.DeepEqual(*mounts, want) {
		t.Fatalf("mounts = %#v, want %#v", *mounts, want)
	}
}

func TestSystemMounterMountsXFSWithNoUUID(t *testing.T) {
	m, mounts, _ := newTestMounter(t, blkidRunner("xfs\n", nil), nil, nil)

	if _, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot"); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	if got := (*mounts)[0]; got.fstype != "xfs" || got.data != "nouuid" {
		t.Fatalf("mount fstype, data = %q, %q, want xfs, nouuid", got.fstype, got.data)
	}
}

func TestSystemMounterMountFailsWithoutFilesystem(t *testing.T) {
	blkidErr := errors.New("blkid failed")
	tests := []struct {
		name    string
		runner  *fakeRunner
		wantErr string
	}{
		{name: "blkid fails", runner: blkidRunner("", blkidErr), wantErr: "detect filesystem"},
		{name: "no filesystem", runner: blkidRunner(" \n", nil), wantErr: "no filesystem detected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, mounts, _ := newTestMounter(t, tt.runner, nil, nil)

			_, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Mount() error = %v, want %q", err, tt.wantErr)
			}
			if len(*mounts) != 0 {
				t.Fatalf("mounts = %#v, want none", *mounts)
			}
			if dirs := mountDirs(t, m); len(dirs) != 0 {
				t.Fatalf("mount directories = %v, want none", dirs)
			}
		})
	}
}

func TestSystemMounterRemovesDirectoryWhenMountFails(t *testing.T) {
	mountErr := errors.New("mount failed")
	m, _, _ := newTestMounter(t, blkidRunner("ext4\n", nil), mountErr, nil)

	_, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot")
	if !errors.Is(err, mountErr) {
		t.Fatalf("Mount() error = %v, want mount error", err)
	}
	if dirs := mountDirs(t, m); len(dirs) != 0 {
		t.Fatalf("mount directories = %v, want none", dirs)
	}
}

func TestSystemMounterUnmountRemovesDirectory(t *testing.T) {
	m, _, unmounts := newTestMounter(t, blkidRunner("ext4\n", nil), nil, nil)
	path, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := m.Unmount(path); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if want := []string{path}; !reflect.DeepEqual(*unmounts, want) {
		t.Fatalf("unmounts = %#v, want %#v", *unmounts, want)
	}
	if dirs := mountDirs(t, m); len(dirs) != 0 {
		t.Fatalf("mount directories = %v, want none", dirs)
	}
}

func TestSystemMounterUnmountKeepsDirectoryWhenUnmountFails(t *testing.T) {
	unmountErr := errors.New("target is busy")
	m, _, _ := newTestMounter(t, blkidRunner("ext4\n", nil), nil, unmountErr)
	path, err := m.Mount(context.Background(), "/dev/vg0/home_backup_snapshot")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	if err := m.Unmount(path); !errors.Is(err, unmountErr) {
		t.Fatalf("Unmount() error = %v, want unmount error", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("mount directory after failed unmount: %v", err)
	}
}
