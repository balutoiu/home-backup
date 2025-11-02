package backup

import (
	"github.com/balutoiu/home-backup/pkg/config"

	log "github.com/sirupsen/logrus"
)

func CreateBackups(config *config.Config) error {
	for _, backup := range config.Backups {
		log.Debugf("creating backup from %s (%s) to %s (%s)", backup.Source.Type, backup.Source.Params, backup.Destination.Type, backup.Destination.Params)
		if err := createSingleBackup(backup); err != nil {
			return err
		}
	}
	return nil
}

func createSingleBackup(backup config.Backup) error {
	log.Debug("setup source backup handler")
	srcBackup, err := NewSourceBackup(backup.Source)
	if err != nil {
		return err
	}
	log.Debug("setup destination backup handler")
	destBackup, err := NewDestinationBackup(backup.Destination)
	if err != nil {
		return err
	}
	log.Debug("setting up source backup (e.g., creating volume snapshot)")
	backupPath, err := srcBackup.Prepare()
	if err != nil {
		return err
	}
	defer func() {
		if err := srcBackup.Cleanup(); err != nil {
			log.Warnf("failed to clean up source backup %s with params (%s): %v", backup.Source.Type, backup.Source.Params, err)
		}
	}()
	log.Debug("creating backup at destination")
	if err := destBackup.Create(backupPath); err != nil {
		return err
	}
	return nil
}
