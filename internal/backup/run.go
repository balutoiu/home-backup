// Package backup defines technology-independent backup workflows.
package backup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const releaseTimeout = 2 * time.Minute

// Backup pairs a source with the destination it is backed up to.
type Backup struct {
	// Label names the backup in errors, e.g. "lvm vg0/home".
	Label       string
	Source      Source
	Destination Destination
}

// Run performs every backup in order. A failed backup does not stop the
// ones after it; cancellation does. All errors are joined.
func Run(ctx context.Context, backups []Backup) error {
	var errs []error
	for i, b := range backups {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("stop before %s: %w", name(i, b), err))
			break
		}
		if err := runOne(ctx, b); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name(i, b), err))
		}
	}
	return errors.Join(errs...)
}

// name identifies the backup at index i in errors.
func name(i int, b Backup) string {
	if b.Label == "" {
		return fmt.Sprintf("backup %d", i+1)
	}
	return fmt.Sprintf("backup %d (%s)", i+1, b.Label)
}

// runOne opens the source, backs it up, and releases the acquired input.
func runOne(ctx context.Context, b Backup) (retErr error) {
	input, err := b.Source.Open(ctx)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer cancel()
		if err := input.Release(releaseCtx); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("release source: %w", err))
		}
	}()

	if err := b.Destination.Backup(ctx, input.Path()); err != nil {
		return fmt.Errorf("backup destination: %w", err)
	}
	return nil
}
