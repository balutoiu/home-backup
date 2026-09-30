// Package directory provides a backup source for an existing directory.
package directory

import (
	"context"
	"fmt"
	"os"

	"github.com/balutoiu/home-backup/internal/backup"
)

// Source opens an existing directory as backup input.
type Source struct {
	path string
}

// NewSource constructs a directory source for path.
func NewSource(path string) *Source { return &Source{path: path} }

// Open validates the directory and returns it as backup input.
func (s *Source) Open(ctx context.Context) (backup.Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return nil, fmt.Errorf("stat directory source %q: %w", s.path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("directory source %q is not a directory", s.path)
	}
	return input{path: s.path}, nil
}

type input struct {
	path string
}

func (i input) Path() string                  { return i.path }
func (i input) Release(context.Context) error { return nil }

var _ backup.Source = (*Source)(nil)
var _ backup.Input = input{}
