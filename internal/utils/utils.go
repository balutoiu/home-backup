package utils

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	mount_utils "k8s.io/mount-utils"
	util_exec "k8s.io/utils/exec"
)

type ExternalCommand struct {
	Command      []string
	CWD          string
	ReturnOutput bool
}

// ExecCommand executes an external command based on the provided ExternalCommand struct.
func ExecCommand(extCmd ExternalCommand) (string, error) {
	cmd := exec.Command(extCmd.Command[0], extCmd.Command[1:]...)
	if extCmd.CWD != "" {
		cmd.Dir = extCmd.CWD
	}
	log.Debug(strings.Join(extCmd.Command, " "))
	if extCmd.ReturnOutput {
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	return "", err
}

// GetExitCode retrieves the exit code from an error returned by exec.Command.
func GetExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	// Non-exit error (e.g., command not found)
	return -1
}

// MountDevice mounts the device at the given path and returns the mount point.
func MountDevice(devicePath string) (string, error) {
	log.Debugf("detecting filesystem type for device %s", devicePath)
	mounter := mount_utils.SafeFormatAndMount{Interface: mount_utils.New(""), Exec: util_exec.New()}
	fsType, err := mounter.GetDiskFormat(devicePath)
	if err != nil {
		return "", err
	}
	log.Debugf("creating temporary directory for mounting device %s", devicePath)
	tmpDir, err := os.MkdirTemp("", "lvm-backup_*")
	if err != nil {
		return "", err
	}
	mountOptions := []string{"ro"}
	switch fsType {
	case "xfs":
		mountOptions = append(mountOptions, "nouuid")
	}
	log.Debugf("mounting device %s", devicePath)
	err = mounter.Mount(devicePath, tmpDir, fsType, mountOptions)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpDir, nil
}

// UnmountDevice unmounts the device at the given path.
func UnmountDevice(path string) error {
	log.Debugf("unmounting: %s", path)
	return mount_utils.New("").Unmount(path)
}

// FileExists checks if a file exists at the given path.
func FileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		// File does not exist (or some other error)
		return false
	}
	// File exists
	return true
}
