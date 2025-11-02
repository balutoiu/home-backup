package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/pkg/config"
)

type SourceBackup interface {
	Prepare() (backupPath string, err error)
	Cleanup() (err error)
}

func NewSourceBackup(srcCfg config.Source) (SourceBackup, error) {
	var srcBackup SourceBackup
	var err error
	switch srcCfg.Type {
	case config.TypeLVM:
		srcBackup, err = NewLVMSourceBackup(srcCfg.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create LVM source backup: %v", err)
		}
	case config.TypeDirectory:
		srcBackup, err = NewDirectorySourceBackup(srcCfg.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create Directory source backup: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported source backup type: %s", srcCfg.Type)
	}
	return srcBackup, nil
}
