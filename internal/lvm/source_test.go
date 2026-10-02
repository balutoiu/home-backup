//go:build linux

package lvm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/balutoiu/home-backup/internal/command"
)

type fakeRunner struct {
	specs   []command.Spec
	results []command.Result
	errs    []error
}

func (f *fakeRunner) Run(_ context.Context, spec command.Spec) (command.Result, error) {
	f.specs = append(f.specs, spec)
	index := len(f.specs) - 1
	var result command.Result
	if index < len(f.results) {
		result = f.results[index]
	}
	var err error
	if index < len(f.errs) {
		err = f.errs[index]
	}
	return result, err
}

type fakeMounter struct {
	path       string
	mountErr   error
	unmountErr error
	mounted    []string
	unmounted  []string
}

func (f *fakeMounter) Mount(_ context.Context, device string) (string, error) {
	f.mounted = append(f.mounted, device)
	return f.path, f.mountErr
}

func (f *fakeMounter) Unmount(path string) error {
	f.unmounted = append(f.unmounted, path)
	return f.unmountErr
}

func TestSourceOpenRequiresRoot(t *testing.T) {
	runner := &fakeRunner{}
	source := NewSource(Config{VGName: "vg0", LVName: "home"}, Dependencies{
		Runner:  runner,
		Mounter: &fakeMounter{},
		EUID:    func() int { return 1000 },
	})

	_, err := source.Open(context.Background())
	if err == nil || !strings.Contains(err.Error(), "root privileges") {
		t.Fatalf("Open() error = %v", err)
	}
	if len(runner.specs) != 0 {
		t.Fatalf("commands = %#v, want none", runner.specs)
	}
}

func TestSourceOpenAndRelease(t *testing.T) {
	runner := &fakeRunner{}
	mounter := &fakeMounter{path: "/mnt/lvm-backup-1"}
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "10G"}, Dependencies{
		Runner:  runner,
		Mounter: mounter,
		EUID:    func() int { return 0 },
	})

	input, err := source.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if input.Path() != mounter.path {
		t.Fatalf("Path() = %q, want %q", input.Path(), mounter.path)
	}
	if err := input.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	wantSpecs := []command.Spec{
		{Name: "sync"},
		{
			Name: "lvcreate",
			Args: []string{
				"--snapshot",
				"--size", "10G",
				"--name", "home_backup_snapshot",
				"/dev/vg0/home",
			},
		},
		{Name: "lvremove", Args: []string{"--force", "/dev/vg0/home_backup_snapshot"}},
	}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
	if want := []string{"/dev/vg0/home_backup_snapshot"}; !reflect.DeepEqual(mounter.mounted, want) {
		t.Fatalf("mounted = %#v, want %#v", mounter.mounted, want)
	}
	if want := []string{mounter.path}; !reflect.DeepEqual(mounter.unmounted, want) {
		t.Fatalf("unmounted = %#v, want %#v", mounter.unmounted, want)
	}
}

func TestSourceOpenRollsBackSnapshotWhenMountFails(t *testing.T) {
	mountErr := errors.New("mount failed")
	runner := &fakeRunner{}
	mounter := &fakeMounter{mountErr: mountErr}
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "2G"}, Dependencies{
		Runner:  runner,
		Mounter: mounter,
		EUID:    func() int { return 0 },
	})

	_, err := source.Open(context.Background())
	if !errors.Is(err, mountErr) {
		t.Fatalf("Open() error = %v, want mount error", err)
	}
	wantSpecs := []command.Spec{
		{Name: "sync"},
		{
			Name: "lvcreate",
			Args: []string{
				"--snapshot",
				"--size", "2G",
				"--name", "home_backup_snapshot",
				"/dev/vg0/home",
			},
		},
		{Name: "lvremove", Args: []string{"--force", "/dev/vg0/home_backup_snapshot"}},
	}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
}
