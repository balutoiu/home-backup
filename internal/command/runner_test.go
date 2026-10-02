package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("HOME_BACKUP_HELPER") != "1" {
		return
	}
	if os.Getenv("HOME_BACKUP_HELPER_WAIT") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if _, err := io.WriteString(os.Stdout, "standard output"); err != nil {
		os.Exit(2)
	}
	if _, err := io.WriteString(os.Stderr, "standard error"); err != nil {
		os.Exit(2)
	}
	if os.Getenv("HOME_BACKUP_HELPER_SUCCESS") == "1" {
		os.Exit(0)
	}
	os.Exit(7)
}

func TestRunnerPreservesContextCancellation(t *testing.T) {
	runner := NewRunner(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.Run(ctx, Spec{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"HOME_BACKUP_HELPER=1", "HOME_BACKUP_HELPER_WAIT=1"},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context deadline exceeded", err)
	}
}

func TestRunnerReturnsStructuredExitError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := NewRunner(logger)
	result, err := runner.Run(context.Background(), Spec{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"HOME_BACKUP_HELPER=1"},
	})
	if result.ExitCode != 7 || result.Stdout != "standard output" || result.Stderr != "standard error" {
		t.Fatalf("result = %#v", result)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerReturnsCapturedSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := NewRunner(logger)
	result, err := runner.Run(context.Background(), Spec{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"HOME_BACKUP_HELPER=1", "HOME_BACKUP_HELPER_SUCCESS=1"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || result.Stdout != "standard output" || result.Stderr != "standard error" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunnerLogsArgsButNotEnv(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	runner := NewRunner(logger)
	_, err := runner.Run(context.Background(), Spec{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"HOME_BACKUP_HELPER=1", "HOME_BACKUP_HELPER_SUCCESS=1", "HOME_BACKUP_SECRET=hunter2"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(logs.String(), "-test.run=TestHelperProcess") {
		t.Fatalf("logs = %q, want the command args", logs.String())
	}
	if strings.Contains(logs.String(), "hunter2") {
		t.Fatalf("logs = %q, want no env values", logs.String())
	}
}

func TestRunnerRejectsEmptyName(t *testing.T) {
	runner := NewRunner(nil)
	result, err := runner.Run(context.Background(), Spec{})
	if err == nil || result.ExitCode != -1 {
		t.Fatalf("Run() result = %#v, error = %v", result, err)
	}
}
