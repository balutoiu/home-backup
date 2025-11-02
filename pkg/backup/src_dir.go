package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/pkg/config"
)

func NewDirectorySourceBackup(params map[string]string) (SourceBackup, error) {
	dirParams := config.SrcDirectoryParams{}
	if err := dirParams.ParseParams(params); err != nil {
		return nil, fmt.Errorf("error parsing Directory source backup params: %v", err)
	}
	return &DirectorySourceBackup{Params: dirParams}, nil
}

type DirectorySourceBackup struct {
	Params config.SrcDirectoryParams
}

func (d *DirectorySourceBackup) Prepare() (string, error) {
	// just return the path as is
	return d.Params.Path, nil
}

func (d *DirectorySourceBackup) Cleanup() error {
	// do not cleanup anything
	return nil
}
