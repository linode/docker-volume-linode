package main

import (
	"os/exec"

	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	formatFSType = "ext4"
)

// Format calls mke2fs on path
func Format(path string) error {
	cmd := exec.Command("mke2fs", "-t", formatFSType, path)
	stdOutAndErr, err := cmd.CombinedOutput()
	log.Debugf("Mke2fs Output:\n%s", stdOutAndErr)
	return err
}

// Mount mounts device to mountpoint
func Mount(device string, mountpoint string) error {
	log.Debugf("calling mount %s %s", device, mountpoint)
	cmd := exec.Command("mount", device, mountpoint)
	output, err := cmd.CombinedOutput()
	log.Debugf("Mount Output:\n%s", string(output))
	return err
}

// Umount calls umount command
func Umount(mountpoint string) error {
	cmd := exec.Command("umount", mountpoint)
	output, err := cmd.CombinedOutput()
	log.Debugf("Umount Output:\n%s", string(output))
	return err
}

// GetFSType returns the filesystem type from a block device
// function based on https://github.com/yholkamp/ovh-docker-volume-plugin/blob/master/utils.go
func GetFSType(device string) string {
	log.Infof("GetFSType(%s)", device)
	fsType := ""
	out, err := exec.Command("blkid", device).CombinedOutput()
	if err != nil {
		return fsType
	}

	if strings.Contains(string(out), "TYPE=") {
		for _, v := range strings.Split(string(out), " ") {
			if strings.Contains(v, "TYPE=") {
				fsType = strings.Split(v, "=")[1]
				fsType = strings.Replace(fsType, "\"", "", -1)
			}
		}
	}

	log.Infof("GetFSType(): %s", fsType)
	return fsType
}
