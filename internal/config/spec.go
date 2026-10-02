// Package config decodes and validates home-backup configuration.
package config

import "fmt"

// SourceKind identifies a configured backup source.
type SourceKind string

const (
	// SourceDirectory selects a directory source.
	SourceDirectory SourceKind = "directory"
	// SourceLVM selects an LVM source.
	SourceLVM SourceKind = "lvm"
)

// DestinationKind identifies a configured backup destination.
type DestinationKind string

const (
	// DestinationRestic selects a Restic destination.
	DestinationRestic DestinationKind = "restic"
)

const (
	// DefaultResticKeepLast is the default number of snapshots retained.
	DefaultResticKeepLast = 10
	// DefaultResticGroupBy is the default Restic snapshot grouping.
	DefaultResticGroupBy = "host"
)

// Config contains all configured backups.
type Config struct {
	Backups []Backup
}

// Backup describes one source-to-destination backup.
type Backup struct {
	Source      Source
	Destination Destination
}

// Source is a typed source variant.
type Source struct {
	Kind      SourceKind
	Directory *DirectorySource
	LVM       *LVMSource
}

// String describes the source by its type and identity, e.g. "lvm vg0/home".
func (s Source) String() string {
	switch s.Kind {
	case SourceDirectory:
		if s.Directory != nil {
			return fmt.Sprintf("%s %s", s.Kind, s.Directory.Path)
		}
	case SourceLVM:
		if s.LVM != nil {
			return fmt.Sprintf("%s %s/%s", s.Kind, s.LVM.VGName, s.LVM.LVName)
		}
	}
	return string(s.Kind)
}

// DirectorySource configures a directory source.
type DirectorySource struct {
	Path string
}

// LVMSource configures an LVM source.
type LVMSource struct {
	VGName string
	LVName string
}

// Destination is a typed destination variant.
type Destination struct {
	Kind   DestinationKind
	Restic *ResticDestination
}

// ResticDestination configures a Restic destination.
type ResticDestination struct {
	Repo     string
	KeepLast int
	GroupBy  string
}
