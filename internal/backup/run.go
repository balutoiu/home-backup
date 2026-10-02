// Package backup defines technology-independent backup workflows.
package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const releaseTimeout = 2 * time.Minute

// Backup pairs a source with the destination it is backed up to.
type Backup struct {
	// Label names the backup in errors and logs, e.g. "lvm vg0/home".
	Label       string
	Source      Source
	Destination Destination
}

// Run performs every backup in order, logging its progress. A failed backup
// does not stop the ones after it; cancellation does. All errors are joined.
func Run(ctx context.Context, logger *slog.Logger, backups []Backup) error {
	start := time.Now()
	logger.InfoContext(ctx, "run started", "backups", len(backups))
	var errs []error
	failed := 0
	for i, b := range backups {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("stop before %s: %w", name(i, b), err))
			logger.WarnContext(ctx, "run cancelled", "skipped", len(backups)-i)
			break
		}
		attrs := logAttrs(i, b)
		logger.LogAttrs(ctx, slog.LevelInfo, "backup started", attrs...)
		backupStart := time.Now()
		err := runOne(ctx, b)
		attrs = append(attrs, slog.Duration("duration", since(backupStart)))
		if err != nil {
			failed++
			errs = append(errs, fmt.Errorf("%s: %w", name(i, b), err))
			logger.LogAttrs(ctx, slog.LevelError, "backup failed", append(attrs, slog.Any("error", err))...)
			continue
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "backup finished", attrs...)
	}
	level := slog.LevelInfo
	if len(errs) > 0 {
		level = slog.LevelError
	}
	logger.Log(ctx, level, "run finished", "backups", len(backups), "failed", failed, "duration", since(start))
	return errors.Join(errs...)
}

// name identifies the backup at index i in errors.
func name(i int, b Backup) string {
	if b.Label == "" {
		return fmt.Sprintf("backup %d", i+1)
	}
	return fmt.Sprintf("backup %d (%s)", i+1, b.Label)
}

// logAttrs identifies the backup at index i in logs.
func logAttrs(i int, b Backup) []slog.Attr {
	attrs := []slog.Attr{slog.Int("backup", i+1)}
	if b.Label != "" {
		attrs = append(attrs, slog.String("source", b.Label))
	}
	return attrs
}

// since returns the time elapsed since start, rounded for logs.
func since(start time.Time) time.Duration {
	return time.Since(start).Round(time.Millisecond)
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
