// Package config decodes and validates home-backup configuration.
package config

// Config contains all configured backups.
type Config struct {
	Backups []Backup
}

// Backup describes one source-to-destination backup.
type Backup struct {
	Source      Source
	Destination Destination
}

// Source is a configured source: DirectorySource or LVMSource.
type Source interface {
	// String describes the source by its type and identity, e.g. "lvm vg0/home".
	String() string
	isSource()
}

// Destination is a configured destination: ResticDestination.
type Destination interface {
	isDestination()
}
