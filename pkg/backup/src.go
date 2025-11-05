package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/pkg/config"
)

type SourceBackup interface {
	Prepare() (backupPath string, err error)
	Cleanup() (err error)
}

func NewSourceBackup(params map[string]string) (SourceBackup, error) {
	if _, ok := params["type"]; !ok {
		return nil, fmt.Errorf("missing source backup 'type' parameter")
	}
	var srcBackup SourceBackup
	var err error
	switch params["type"] {
	case config.TypeLVM:
		srcBackup, err = NewLVMSourceBackup(params)
		if err != nil {
			return nil, fmt.Errorf("failed to create LVM source backup: %v", err)
		}
	case config.TypeDirectory:
		srcBackup, err = NewDirectorySourceBackup(params)
		if err != nil {
			return nil, fmt.Errorf("failed to create Directory source backup: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported source backup type: %s", params["type"])
	}
	return srcBackup, nil
}
