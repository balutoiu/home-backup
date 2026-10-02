package backup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type sourceFunc func(context.Context) (Input, error)

func (f sourceFunc) Open(ctx context.Context) (Input, error) { return f(ctx) }

type destinationFunc func(context.Context, string) error

func (f destinationFunc) Backup(ctx context.Context, path string) error { return f(ctx, path) }

type fakeInput struct {
	path    string
	release func(context.Context) error
}

func (f *fakeInput) Path() string { return f.path }

func (f *fakeInput) Release(ctx context.Context) error { return f.release(ctx) }

func openPath(path string) Source {
	return sourceFunc(func(context.Context) (Input, error) {
		return &fakeInput{path: path, release: func(context.Context) error { return nil }}, nil
	})
}

func backupSucceeds() Destination {
	return destinationFunc(func(context.Context, string) error { return nil })
}

func TestRunOrdersLifecycle(t *testing.T) {
	var calls []string
	source := func(path string) Source {
		return sourceFunc(func(context.Context) (Input, error) {
			calls = append(calls, "open:"+path)
			return &fakeInput{path: path, release: func(context.Context) error {
				calls = append(calls, "release:"+path)
				return nil
			}}, nil
		})
	}
	destination := destinationFunc(func(_ context.Context, path string) error {
		calls = append(calls, "backup:"+path)
		return nil
	})

	err := Run(context.Background(), []Backup{
		{Source: source("/first"), Destination: destination},
		{Source: source("/second"), Destination: destination},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{
		"open:/first", "backup:/first", "release:/first",
		"open:/second", "backup:/second", "release:/second",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestRunJoinsBackupAndReleaseErrors(t *testing.T) {
	backupErr := errors.New("backup failed")
	releaseErr := errors.New("release failed")

	err := Run(context.Background(), []Backup{{
		Source: sourceFunc(func(context.Context) (Input, error) {
			return &fakeInput{path: "/snapshot", release: func(context.Context) error { return releaseErr }}, nil
		}),
		Destination: destinationFunc(func(context.Context, string) error { return backupErr }),
	}})
	if !errors.Is(err, backupErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("Run() error = %v, want both errors", err)
	}
}

func TestRunReleasesWithUncancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	released := false

	err := Run(ctx, []Backup{{
		Source: sourceFunc(func(context.Context) (Input, error) {
			return &fakeInput{path: "/snapshot", release: func(ctx context.Context) error {
				if err := ctx.Err(); err != nil {
					t.Fatalf("release context error = %v", err)
				}
				released = true
				return nil
			}}, nil
		}),
		Destination: destinationFunc(func(context.Context, string) error {
			cancel()
			return context.Canceled
		}),
	}})
	if !errors.Is(err, context.Canceled) || !released {
		t.Fatalf("Run() error = %v, released = %v", err, released)
	}
}

func TestRunContinuesAfterFailedBackup(t *testing.T) {
	backupErr := errors.New("backup failed")
	secondRan := false

	err := Run(context.Background(), []Backup{
		{
			Source:      openPath("/first"),
			Destination: destinationFunc(func(context.Context, string) error { return backupErr }),
		},
		{
			Source: openPath("/second"),
			Destination: destinationFunc(func(context.Context, string) error {
				secondRan = true
				return nil
			}),
		},
	})
	if !errors.Is(err, backupErr) || !secondRan {
		t.Fatalf("Run() error = %v, secondRan = %v", err, secondRan)
	}
	if !strings.Contains(err.Error(), "backup 1") {
		t.Fatalf("Run() error = %v, want backup index", err)
	}
}

func TestRunContinuesAfterFailedOpen(t *testing.T) {
	openErr := errors.New("open failed")
	secondRan := false

	err := Run(context.Background(), []Backup{
		{
			Source:      sourceFunc(func(context.Context) (Input, error) { return nil, openErr }),
			Destination: backupSucceeds(),
		},
		{
			Source: openPath("/second"),
			Destination: destinationFunc(func(context.Context, string) error {
				secondRan = true
				return nil
			}),
		},
	})
	if !errors.Is(err, openErr) || !secondRan {
		t.Fatalf("Run() error = %v, secondRan = %v", err, secondRan)
	}
}

func TestRunStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	secondOpened := false

	err := Run(ctx, []Backup{
		{
			Source: openPath("/first"),
			Destination: destinationFunc(func(context.Context, string) error {
				cancel()
				return nil
			}),
		},
		{
			Source: sourceFunc(func(context.Context) (Input, error) {
				secondOpened = true
				return nil, errors.New("unexpected open")
			}),
			Destination: backupSucceeds(),
		},
	})
	if !errors.Is(err, context.Canceled) || secondOpened {
		t.Fatalf("Run() error = %v, secondOpened = %v", err, secondOpened)
	}
	if !strings.Contains(err.Error(), "stop before backup 2") {
		t.Fatalf("Run() error = %v, want stopped backup index", err)
	}
}

func TestRunNamesFailedBackupByLabel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	err := Run(ctx, []Backup{
		{
			Label:  "lvm vg0/home",
			Source: openPath("/snapshot"),
			Destination: destinationFunc(func(context.Context, string) error {
				cancel()
				return errors.New("backup failed")
			}),
		},
		{Label: "directory /srv/photos", Source: openPath("/srv/photos"), Destination: backupSucceeds()},
	})
	for _, want := range []string{
		"backup 1 (lvm vg0/home): backup destination: backup failed",
		"stop before backup 2 (directory /srv/photos): context canceled",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Run() error = %v, want substring %q", err, want)
		}
	}
}
