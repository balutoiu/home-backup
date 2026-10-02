package config

import (
	"errors"
	"fmt"
)

func (c Config) validate() error {
	if len(c.Backups) == 0 {
		return errors.New("configuration requires at least one backup")
	}
	for i, job := range c.Backups {
		if err := job.Source.validate(); err != nil {
			return fmt.Errorf("backup %d source: %w", i+1, err)
		}
		if err := job.Destination.validate(); err != nil {
			return fmt.Errorf("backup %d destination: %w", i+1, err)
		}
	}
	return nil
}

func (s Source) validate() error {
	switch s.Kind {
	case SourceDirectory:
		if s.Directory == nil || s.LVM != nil {
			return errors.New("directory source variant is inconsistent")
		}
		if s.Directory.Path == "" {
			return errors.New("directory source path is required")
		}
	case SourceLVM:
		if s.LVM == nil || s.Directory != nil {
			return errors.New("LVM source variant is inconsistent")
		}
		if s.LVM.VGName == "" {
			return errors.New("LVM source vg_name is required")
		}
		if s.LVM.LVName == "" {
			return errors.New("LVM source lv_name is required")
		}
	default:
		return fmt.Errorf("unsupported source type %q", s.Kind)
	}
	return nil
}

func (d Destination) validate() error {
	if d.Kind != DestinationRestic {
		return fmt.Errorf("unsupported destination type %q", d.Kind)
	}
	if d.Restic == nil {
		return errors.New("restic destination variant is inconsistent")
	}
	if d.Restic.Repo == "" {
		return errors.New("restic destination repo is required")
	}
	if d.Restic.KeepLast < 0 {
		return errors.New("restic destination keep_last cannot be negative")
	}
	return nil
}
