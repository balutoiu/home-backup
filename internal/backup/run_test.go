package backup

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

var discard = slog.New(slog.DiscardHandler)

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

	err := Run(context.Background(), discard, []Backup{
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

	err := Run(context.Background(), discard, []Backup{{
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

	err := Run(ctx, discard, []Backup{{
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

	err := Run(context.Background(), discard, []Backup{
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

	err := Run(context.Background(), discard, []Backup{
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

	err := Run(ctx, discard, []Backup{
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

	err := Run(ctx, discard, []Backup{
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

// captureLogs returns a logger and a function that reads its lines, with the
// time dropped and durations masked.
func captureLogs() (*slog.Logger, func() []string) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				return slog.Attr{}
			case "duration":
				return slog.String("duration", "X")
			}
			return a
		},
	}))
	return logger, func() []string { return strings.Split(strings.TrimSpace(buf.String()), "\n") }
}

func TestRunLogsProgress(t *testing.T) {
	logger, lines := captureLogs()

	err := Run(context.Background(), logger, []Backup{
		{Label: "directory /srv/photos", Source: openPath("/srv/photos"), Destination: backupSucceeds()},
		{
			Label:       "lvm vg0/home",
			Source:      sourceFunc(func(context.Context) (Input, error) { return nil, errors.New("no space") }),
			Destination: backupSucceeds(),
		},
		{Source: openPath("/etc"), Destination: backupSucceeds()},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
	want := []string{
		`level=INFO msg="run started" backups=3`,
		`level=INFO msg="backup started" backup=1 source="directory /srv/photos"`,
		`level=INFO msg="backup finished" backup=1 source="directory /srv/photos" duration=X`,
		`level=INFO msg="backup started" backup=2 source="lvm vg0/home"`,
		`level=ERROR msg="backup failed" backup=2 source="lvm vg0/home" duration=X error="open source: no space"`,
		`level=INFO msg="backup started" backup=3`,
		`level=INFO msg="backup finished" backup=3 duration=X`,
		`level=ERROR msg="run finished" backups=3 failed=1 duration=X`,
	}
	if got := lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("logs =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRunLogsSuccessfulRunAtInfo(t *testing.T) {
	logger, lines := captureLogs()

	if err := Run(context.Background(), logger, []Backup{{Source: openPath("/etc"), Destination: backupSucceeds()}}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := lines()
	if want := `level=INFO msg="run finished" backups=1 failed=0 duration=X`; got[len(got)-1] != want {
		t.Fatalf("last log = %q, want %q", got[len(got)-1], want)
	}
}

func TestRunLogsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger, lines := captureLogs()

	err := Run(ctx, logger, []Backup{
		{
			Source: openPath("/first"),
			Destination: destinationFunc(func(context.Context, string) error {
				cancel()
				return nil
			}),
		},
		{Source: openPath("/second"), Destination: backupSucceeds()},
		{Source: openPath("/third"), Destination: backupSucceeds()},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want cancellation", err)
	}
	want := []string{
		`level=INFO msg="run started" backups=3`,
		`level=INFO msg="backup started" backup=1`,
		`level=INFO msg="backup finished" backup=1 duration=X`,
		`level=WARN msg="run cancelled" skipped=2`,
		`level=ERROR msg="run finished" backups=3 failed=0 duration=X`,
	}
	if got := lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("logs =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
