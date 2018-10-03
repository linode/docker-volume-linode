package main

import (
	"context"
	"errors"
	"math"
	"os"
	"path"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/linode/linodego"
	log "github.com/sirupsen/logrus"
)

// labelToMountPoint gets the mount-point for a volume
func labelToMountPoint(volumeLabel string) string {
	return path.Join(MountRoot, volumeLabel)
}

// waitForDeviceFileExists waits until path devicePath becomes available or
// times out.
func waitForDeviceFileExists(devicePath string, waitSeconds int) error {
	return waitForCondition(waitSeconds, 1, func() bool {
		// found, then break
		if _, err := os.Stat(devicePath); !os.IsNotExist(err) {
			return true // condition met
		}
		log.Infof("Waiting for device %s to be available", devicePath)
		return false
	})
}

func waitForLinodeVolumeDetachment(linodeAPI linodego.Client, volumeID int) error {
	// Wait for linode to have the volume detached
	return waitForCondition(180, 2, func() bool {
		v, err := linodeAPI.GetVolume(context.Background(), volumeID)
		if err != nil {
			log.Error(err)
			return false
		}

		if v.LinodeID == nil {
			return true // detached
		}

		return false
	})
}

// waitForCondition Waits until condition returns true timeout is reached.  If timeout is
// reached it returns error.
func waitForCondition(waitSeconds int, intervalSeconds int, check func() bool) error {
	loops := int(math.Ceil(float64(waitSeconds) / float64(intervalSeconds)))
	for i := 0; i < loops; i++ {
		if check() {
			return nil
		}
		time.Sleep(time.Second * time.Duration(intervalSeconds))
	}
	return errors.New("waitForCondition timeout")
}

// linodeVolumeToDockerVolume converts a linode volume to a docker volume
func linodeVolumeToDockerVolume(lv *linodego.Volume) *volume.Volume {
	v := &volume.Volume{
		Name:       lv.Label,
		Mountpoint: labelToMountPoint(lv.Label),
		CreatedAt:  lv.Created.Format(time.RFC3339),
		Status:     make(map[string]interface{}),
	}
	return v
}
