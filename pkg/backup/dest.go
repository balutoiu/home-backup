package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/pkg/config"
)

type DestinationBackup interface {
	Create(backupPath string) (err error)
}

func NewDestinationBackup(params map[string]string) (DestinationBackup, error) {
	if _, ok := params["type"]; !ok {
		return nil, fmt.Errorf("missing destination backup 'type' parameter")
	}
	var destBackup DestinationBackup
	var err error
	switch params["type"] {
	case config.TypeRestic:
		destBackup, err = NewResticDestBackup(params)
		if err != nil {
			return nil, fmt.Errorf("failed to create Restic destination backup: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported destination backup type: %s", params["type"])
	}
	return destBackup, nil
}
