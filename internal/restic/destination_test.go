package restic

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/balutoiu/home-backup/internal/command"
)

type fakeRunner struct {
	specs []command.Spec
	errs  []error
}

func (f *fakeRunner) Run(_ context.Context, spec command.Spec) (command.Result, error) {
	f.specs = append(f.specs, spec)
	index := len(f.specs) - 1
	if index < len(f.errs) {
		return command.Result{}, f.errs[index]
	}
	return command.Result{}, nil
}

func TestDestinationBackupExistingRepository(t *testing.T) {
	runner := &fakeRunner{}
	destination := NewDestination(Config{
		Repo:     "/backups/restic",
		KeepLast: 5,
		GroupBy:  "host",
	}, runner)

	if err := destination.Backup(context.Background(), "/snapshot"); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	env := []string{"RESTIC_REPOSITORY=/backups/restic"}
	want := []command.Spec{
		{Name: "restic", Args: []string{"cat", "config"}, Env: env},
		{Name: "restic", Args: []string{"backup", "."}, Dir: "/snapshot", Env: env},
		{Name: "restic", Args: []string{
			"forget",
			"--group-by", "host",
			"--keep-last", "5",
			"--prune",
		}, Env: env},
	}
	if !reflect.DeepEqual(runner.specs, want) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, want)
	}
}

func TestDestinationBackupInitializesMissingRepository(t *testing.T) {
	missing := &command.ExitError{
		Spec:   command.Spec{Name: "restic"},
		Result: command.Result{ExitCode: 10, Stderr: "repository does not exist"},
		Err:    errors.New("exit status 10"),
	}
	runner := &fakeRunner{errs: []error{missing}}
	destination := NewDestination(Config{
		Repo:     "sftp:backup:/repo",
		KeepLast: 2,
		GroupBy:  "paths",
	}, runner)

	if err := destination.Backup(context.Background(), "/snapshot"); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	env := []string{"RESTIC_REPOSITORY=sftp:backup:/repo"}
	want := []command.Spec{
		{Name: "restic", Args: []string{"cat", "config"}, Env: env},
		{Name: "restic", Args: []string{"init"}, Env: env},
		{Name: "restic", Args: []string{"backup", "."}, Dir: "/snapshot", Env: env},
		{Name: "restic", Args: []string{
			"forget",
			"--group-by", "paths",
			"--keep-last", "2",
			"--prune",
		}, Env: env},
	}
	if !reflect.DeepEqual(runner.specs, want) {
		t.Fatalf("commands = %#v, want %#v", runner.specs, want)
	}
}

func TestDestinationBackupKeepsRepositoryOutOfArgs(t *testing.T) {
	const repo = "rest:https://user:secret@backup.example/repo"
	missing := &command.ExitError{
		Spec:   command.Spec{Name: "restic"},
		Result: command.Result{ExitCode: 10},
		Err:    errors.New("exit status 10"),
	}
	runner := &fakeRunner{errs: []error{missing}}
	destination := NewDestination(Config{Repo: repo, KeepLast: 1, GroupBy: "host"}, runner)

	if err := destination.Backup(context.Background(), "/snapshot"); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	for _, spec := range runner.specs {
		for _, arg := range spec.Args {
			if strings.Contains(arg, "secret") {
				t.Fatalf("command %v has the repository in its args", spec.Args)
			}
		}
	}
}

func TestDestinationBackupStopsOnUnexpectedRepositoryError(t *testing.T) {
	checkErr := errors.New("permission denied")
	runner := &fakeRunner{errs: []error{checkErr}}
	destination := NewDestination(Config{
		Repo:     "/backups/restic",
		KeepLast: 5,
		GroupBy:  "host",
	}, runner)

	err := destination.Backup(context.Background(), "/snapshot")
	if !errors.Is(err, checkErr) {
		t.Fatalf("Backup() error = %v, want check error", err)
	}
	if len(runner.specs) != 1 {
		t.Fatalf("commands = %#v, want repository check only", runner.specs)
	}
}
