package backup

import (
	"fmt"

	"github.com/balutoiu/home-backup/internal/utils"
	"github.com/balutoiu/home-backup/pkg/config"

	log "github.com/sirupsen/logrus"
)

func NewResticDestBackup(params map[string]string) (DestinationBackup, error) {
	resticParams := config.DestResticParams{}
	if err := resticParams.ParseParams(params); err != nil {
		return nil, fmt.Errorf("error parsing Restic destination backup params: %v", err)
	}
	return &ResticDestinationBackup{Params: resticParams}, nil
}

type ResticDestinationBackup struct {
	Params config.DestResticParams
}

func (r *ResticDestinationBackup) Create(backupPath string) error {
	log.Debug("checking if restic repository exists")
	cmd := utils.ExternalCommand{
		Command:      []string{"restic", "--repo", r.Params.Repo, "cat", "config"},
		ReturnOutput: true,
	}
	if _, err := utils.ExecCommand(cmd); err != nil {
		if utils.GetExitCode(err) == 10 {
			log.Debug("restic repository does not exist")
			log.Debug("initializing restic repository")
			cmd = utils.ExternalCommand{
				Command: []string{"restic", "--repo", r.Params.Repo, "init"},
			}
			if _, err := utils.ExecCommand(cmd); err != nil {
				return fmt.Errorf("failed to initialize restic repository: %v", err)
			}
		}
	}
	log.Debug("creating restic backup")
	cmd = utils.ExternalCommand{
		Command: []string{"restic", "--repo", r.Params.Repo, "backup", "."},
		CWD:     backupPath,
	}
	if _, err := utils.ExecCommand(cmd); err != nil {
		return fmt.Errorf("failed to create restic backup: %v", err)
	}
	return nil
}
