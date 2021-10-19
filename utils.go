package main

import (
	"context"
	"errors"
	"math"
	"os"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/linode/linodego"
	log "github.com/sirupsen/logrus"
)

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

func waitForLinodeVolumeDetachment(linodeAPI linodego.Client, volumeID, timeout int) error {
	// Wait for linode to have the volume detached
	return waitForCondition(timeout, 2, func() bool {
		v, err := linodeAPI.GetVolume(context.Background(), volumeID)
		if err != nil {
			log.Error(err)
			return false
		}

		return v.LinodeID == nil
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
func linodeVolumeToDockerVolume(lv linodego.Volume, mp string) *volume.Volume {
	v := &volume.Volume{
		Name:       lv.Label,
		Mountpoint: mp,
		CreatedAt:  lv.Created.Format(time.RFC3339),
		Status:     make(map[string]interface{}),
	}
	return v
}
