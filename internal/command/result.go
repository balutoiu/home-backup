// Package command executes external programs without invoking a shell.
package command

import (
	"fmt"
	"strings"
)

// Spec describes one external command execution.
type Spec struct {
	Name string
	Args []string
	Dir  string
	// Env is never logged, unlike Args, so secrets go here.
	Env []string
}

// Result captures a command's output and exit status.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ExitError describes a command that failed to start or exited unsuccessfully.
type ExitError struct {
	Spec   Spec
	Result Result
	Err    error
}

// Error returns a concise command failure message.
func (e *ExitError) Error() string {
	message := fmt.Sprintf("command %q exited with code %d", e.Spec.Name, e.Result.ExitCode)
	if stderr := strings.TrimSpace(e.Result.Stderr); stderr != "" {
		return message + ": " + stderr
	}
	if e.Err != nil {
		return message + ": " + e.Err.Error()
	}
	return message
}

// Unwrap returns the underlying process error.
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode returns the process exit status, or -1 if it never started.
func (e *ExitError) ExitCode() int { return e.Result.ExitCode }
