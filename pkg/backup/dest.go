package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/pkg/config"
)

type DestinationBackup interface {
	Create(backupPath string) (err error)
}

func NewDestinationBackup(destCfg config.Destination) (DestinationBackup, error) {
	var destBackup DestinationBackup
	var err error
	switch destCfg.Type {
	case config.TypeRestic:
		destBackup, err = NewResticDestBackup(destCfg.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create Restic destination backup: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported destination backup type: %s", destCfg.Type)
	}
	return destBackup, nil
}
