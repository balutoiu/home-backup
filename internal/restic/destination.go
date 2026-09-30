// Package restic provides a Restic backup destination.
package restic

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/balutoiu/home-backup/internal/backup"
	"github.com/balutoiu/home-backup/internal/command"
)

const repositoryNotFoundExitCode = 10

// CommandRunner executes Restic commands.
type CommandRunner interface {
	Run(context.Context, command.Spec) (command.Result, error)
}

// Config defines a Restic repository and its retention policy.
type Config struct {
	Repo     string
	KeepLast int
	GroupBy  string
}

// Destination stores backup inputs in a Restic repository.
type Destination struct {
	config Config
	runner CommandRunner
}

// NewDestination constructs a Restic destination.
func NewDestination(cfg Config, runner CommandRunner) *Destination {
	return &Destination{config: cfg, runner: runner}
}

// Backup creates a snapshot and applies the configured retention policy.
func (d *Destination) Backup(ctx context.Context, path string) error {
	_, err := d.runner.Run(ctx, command.Spec{
		Name: "restic",
		Args: []string{"--repo", d.config.Repo, "cat", "config"},
	})
	if err != nil {
		var exitErr *command.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != repositoryNotFoundExitCode {
			return fmt.Errorf("check Restic repository: %w", err)
		}
		if _, err := d.runner.Run(ctx, command.Spec{
			Name: "restic",
			Args: []string{"--repo", d.config.Repo, "init"},
		}); err != nil {
			return fmt.Errorf("initialize Restic repository: %w", err)
		}
	}

	if _, err := d.runner.Run(ctx, command.Spec{
		Name: "restic",
		Args: []string{"--repo", d.config.Repo, "backup", "."},
		Dir:  path,
	}); err != nil {
		return fmt.Errorf("create Restic backup: %w", err)
	}

	if _, err := d.runner.Run(ctx, command.Spec{
		Name: "restic",
		Args: []string{
			"--repo", d.config.Repo,
			"forget",
			"--group-by", d.config.GroupBy,
			"--keep-last", strconv.Itoa(d.config.KeepLast),
			"--prune",
		},
	}); err != nil {
		return fmt.Errorf("apply Restic retention: %w", err)
	}
	return nil
}

var _ backup.Destination = (*Destination)(nil)
