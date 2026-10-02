//go:build linux

package lvm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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

var discard = slog.New(slog.DiscardHandler)

// errNoVolume is what the fake lvs returns when there is no stale snapshot.
var errNoVolume = errors.New("failed to find logical volume")

var staleCheck = command.Spec{
	Name: "lvs",
	Args: []string{"--noheadings", "--options", "origin", "vg0/home_backup_snapshot"},
}

var removeSnapshot = command.Spec{Name: "lvremove", Args: []string{"--force", "/dev/vg0/home_backup_snapshot"}}

func createSnapshot(size string) command.Spec {
	return command.Spec{
		Name: "lvcreate",
		Args: []string{
			"--snapshot",
			"--size", size,
			"--name", "home_backup_snapshot",
			"/dev/vg0/home",
		},
	}
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
		Logger:  discard,
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
	runner := &fakeRunner{errs: []error{nil, errNoVolume}}
	mounter := &fakeMounter{path: "/mnt/lvm-backup-1"}
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "10G"}, Dependencies{
		Runner:  runner,
		Mounter: mounter,
		EUID:    func() int { return 0 },
		Logger:  discard,
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

	wantSpecs := []command.Spec{{Name: "sync"}, staleCheck, createSnapshot("10G"), removeSnapshot}
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
	runner := &fakeRunner{errs: []error{nil, errNoVolume}}
	mounter := &fakeMounter{mountErr: mountErr}
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "2G"}, Dependencies{
		Runner:  runner,
		Mounter: mounter,
		EUID:    func() int { return 0 },
		Logger:  discard,
	})

	_, err := source.Open(context.Background())
	if !errors.Is(err, mountErr) {
		t.Fatalf("Open() error = %v, want mount error", err)
	}
	wantSpecs := []command.Spec{{Name: "sync"}, staleCheck, createSnapshot("2G"), removeSnapshot}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
}

func TestSourceOpenRemovesStaleSnapshot(t *testing.T) {
	runner := &fakeRunner{results: []command.Result{{}, {Stdout: "  home\n"}}}
	var logs bytes.Buffer
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "10G"}, Dependencies{
		Runner:  runner,
		Mounter: &fakeMounter{path: "/mnt/lvm-backup-1"},
		EUID:    func() int { return 0 },
		Logger:  slog.New(slog.NewTextHandler(&logs, nil)),
	})

	if _, err := source.Open(context.Background()); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	wantSpecs := []command.Spec{{Name: "sync"}, staleCheck, removeSnapshot, createSnapshot("10G")}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
	want := `level=WARN msg="removed stale LVM snapshot" snapshot=/dev/vg0/home_backup_snapshot`
	if !strings.Contains(logs.String(), want) {
		t.Fatalf("logs = %q, want %q", logs.String(), want)
	}
}

func TestSourceOpenFailsOnStaleSnapshotProblems(t *testing.T) {
	tests := []struct {
		name      string
		runner    *fakeRunner
		wantSpecs []command.Spec
		wantErr   string
	}{
		{
			name:      "volume is not a snapshot of the origin",
			runner:    &fakeRunner{results: []command.Result{{}, {Stdout: "  \n"}}},
			wantSpecs: []command.Spec{{Name: "sync"}, staleCheck},
			wantErr:   `LVM volume "/dev/vg0/home_backup_snapshot" exists but is not a snapshot of "home"`,
		},
		{
			name: "stale snapshot cannot be removed",
			runner: &fakeRunner{
				results: []command.Result{{}, {Stdout: "home\n"}},
				errs:    []error{nil, nil, errors.New("logical volume in use")},
			},
			wantSpecs: []command.Spec{{Name: "sync"}, staleCheck, removeSnapshot},
			wantErr:   `remove stale LVM snapshot "/dev/vg0/home_backup_snapshot": logical volume in use`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounter := &fakeMounter{}
			source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "10G"}, Dependencies{
				Runner:  tt.runner,
				Mounter: mounter,
				EUID:    func() int { return 0 },
				Logger:  discard,
			})

			_, err := source.Open(context.Background())
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("Open() error = %v, want %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.runner.specs, tt.wantSpecs) {
				t.Fatalf("commands = %#v, want %#v", tt.runner.specs, tt.wantSpecs)
			}
			if len(mounter.mounted) != 0 {
				t.Fatalf("mounted = %#v, want none", mounter.mounted)
			}
		})
	}
}

func TestSourceReleaseKeepsSnapshotWhenUnmountFails(t *testing.T) {
	unmountErr := errors.New("target is busy")
	runner := &fakeRunner{errs: []error{nil, errNoVolume}}
	source := NewSource(Config{VGName: "vg0", LVName: "home", SnapshotSize: "10G"}, Dependencies{
		Runner:  runner,
		Mounter: &fakeMounter{path: "/mnt/lvm-backup-1", unmountErr: unmountErr},
		EUID:    func() int { return 0 },
		Logger:  discard,
	})
	input, err := source.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := input.Release(context.Background()); !errors.Is(err, unmountErr) {
		t.Fatalf("Release() error = %v, want unmount error", err)
	}
	wantSpecs := []command.Spec{{Name: "sync"}, staleCheck, createSnapshot("10G")}
	if !reflect.DeepEqual(runner.specs, wantSpecs) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, wantSpecs)
	}
}
