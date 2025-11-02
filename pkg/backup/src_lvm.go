package backup

import (
	"fmt"
	"os"

	"github.com/balutoiu/home-backup/internal/utils"
	"github.com/balutoiu/home-backup/pkg/config"

	log "github.com/sirupsen/logrus"
)

const (
	defaultLVMSnapshotSize = "10G"
)

func NewLVMSourceBackup(params map[string]string) (SourceBackup, error) {
	lvmParams := config.SrcLVMParams{}
	if err := lvmParams.ParseParams(params); err != nil {
		return nil, fmt.Errorf("error parsing LVM source backup params: %v", err)
	}
	return &LVMSourceBackup{Params: lvmParams}, nil
}

type LVMSourceBackup struct {
	Params    config.SrcLVMParams
	mountPath string
}

func (lvmSrc *LVMSourceBackup) Prepare() (string, error) {
	log.Debug("syncing filesystem before creating LVM snapshot")
	if _, err := utils.ExecCommand(utils.ExternalCommand{Command: []string{"sync"}}); err != nil {
		return "", fmt.Errorf("error syncing filesystem: %v", err)
	}
	log.Debug("creating LVM snapshot")
	cmd := utils.ExternalCommand{
		Command: []string{
			"lvcreate", "--snapshot",
			"--size", defaultLVMSnapshotSize,
			"--name", lvmSrc.getSnapshotName(),
			lvmSrc.getLVPath(),
		},
	}
	if _, err := utils.ExecCommand(cmd); err != nil {
		return "", fmt.Errorf("error creating LVM snapshot: %v", err)
	}
	log.Debug("mounting LVM snapshot")
	mountPath, err := utils.MountDevice(lvmSrc.getSnapshotPath())
	if err != nil {
		return "", fmt.Errorf("error mounting LVM snapshot: %v", err)
	}
	lvmSrc.mountPath = mountPath
	return lvmSrc.mountPath, nil
}

func (lvmSrc *LVMSourceBackup) Cleanup() error {
	log.Debug("unmounting LVM snapshot")
	if err := utils.UnmountDevice(lvmSrc.getSnapshotPath()); err != nil {
		log.Errorf("failed to unmount device %s: %v", lvmSrc.getSnapshotPath(), err)
	}
	log.Debug("removing LVM snapshot")
	cmd := utils.ExternalCommand{
		Command: []string{"lvremove", "--force", lvmSrc.getSnapshotPath()},
	}
	if _, err := utils.ExecCommand(cmd); err != nil {
		return fmt.Errorf("error removing LVM snapshot: %v", err)
	}
	log.Debugf("removing temporary mount path: %s", lvmSrc.mountPath)
	if utils.FileExists(lvmSrc.mountPath) {
		os.RemoveAll(lvmSrc.mountPath)
	}
	return nil
}

func (lvmSrc *LVMSourceBackup) getLVPath() string {
	return fmt.Sprintf("/dev/%s/%s", lvmSrc.Params.VGName, lvmSrc.Params.LVName)
}

func (lvmSrc *LVMSourceBackup) getSnapshotName() string {
	return fmt.Sprintf("%s_backup_snapshot", lvmSrc.Params.LVName)
}

func (lvmSrc *LVMSourceBackup) getSnapshotPath() string {
	return fmt.Sprintf("/dev/%s/%s", lvmSrc.Params.VGName, lvmSrc.getSnapshotName())
}
