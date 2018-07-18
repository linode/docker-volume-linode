package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/chiefy/linodego"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/log"
)

const (
	mountOptions = "data=ordered,noatime,nodiratime"
	mountFSType  = "ext4"
)

type linodeVolumeDriver struct {
	linodeAPI   linodego.Client
	instanceID  *int
	region      string
	linodeLabel *string
	volumeMap   map[string]chan error
}

// Constructor
func newLinodeVolumeDriver(linodeAPI linodego.Client, region string, linodeLabel *string) linodeVolumeDriver {
	driver := linodeVolumeDriver{linodeAPI: linodeAPI, region: region, linodeLabel: linodeLabel, volumeMap: make(map[string]chan error)}

	if linodeLabel != nil {
		jsonFilter, _ := json.Marshal(map[string]string{"label": *linodeLabel})
		listOpts := linodego.NewListOptions(0, string(jsonFilter))
		linodes, lErr := driver.linodeAPI.ListInstances(listOpts)

		if lErr != nil {
			log.Error("Could not determine Linode instance ID from Linode label %s due to error: %s", *linodeLabel, lErr)
			os.Exit(1)
		} else if len(linodes) != 1 {
			log.Error("Could not determine Linode instance ID from Linode label %s", *linodeLabel)
			os.Exit(1)
		}

		driver.instanceID = &linodes[0].ID
	}

	// @TODO what is the plan for this driver if we are running without being tied to a specific Linode?

	return driver
}

// Get implementation
func (driver linodeVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	log.Info("Get(%s)", req.Name)
	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, log.Err("%s", err)
	}

	if linVol == nil {
		return nil, log.Err("Got a NIL volume. Volume may not exist.")
	}

	vol := linodeVolumeToDockerVolume(linVol)
	resp := &volume.GetResponse{Volume: vol}

	log.Info("Get(): %+v", resp)

	return resp, nil
}

// TODO: Listing not working... will address in other commit
// List implementation
func (driver linodeVolumeDriver) List() (*volume.ListResponse, error) {
	log.Info("List()")

	//
	var volumes []*volume.Volume

	linVols, err := driver.linodeAPI.ListInstanceVolumes(*driver.instanceID, nil)
	if err != nil {
		return nil, log.Err("%s", err)
	}
	log.Debug("Got %d volume count from api", len(linVols))
	for _, linVol := range linVols {
		vol := linodeVolumeToDockerVolume(linVol)
		log.Debug("Volume: %+v", vol)
		volumes = append(volumes, vol)
	}
	log.Info("List() returning %d: %+v", len(volumes), volumes)
	return &volume.ListResponse{Volumes: volumes}, nil
}

// Create implementation
func (driver linodeVolumeDriver) Create(req *volume.CreateRequest) error {
	log.Info("Create(%s)", req.Name)
	var size int
	var err error
	if sizeOpt, ok := req.Options["size"]; ok {
		if size, err = strconv.Atoi(sizeOpt); err != nil {
			return log.Err("Invalid size")
		}
	}
	createOpts := linodego.VolumeCreateOptions{
		Label:    req.Name,
		LinodeID: *driver.instanceID,
		Size:     size,
	}
	vol, err := driver.linodeAPI.CreateVolume(createOpts)
	if err != nil {
		return log.Err("Create(%s) Failed: %s", req.Name, err)
	}

	// add volume to map until it is formatted
	chn := make(chan error, 1)
	driver.volumeMap[req.Name] = chn

	// Format in background to avoid context cancellation on the side of docker
	go func() {
		log.Info("Waiting for volume %s to be attached to linode", req.Name)
		if err := linodego.WaitForVolumeStatus(&driver.linodeAPI, vol.ID, linodego.VolumeActive, 180); err != nil {
			chn <- log.Err("Error returned while waiting for volume state: %s", err)
			return
		}

		log.Info("Wait for kernel to have volume %s as device %s", req.Name, vol.FilesystemPath)
		if err := waitForDeviceFileExists(vol.FilesystemPath, 180); err != nil {
			chn <- log.Err("Error waiting for device(%s) to become available: %s", vol.FilesystemPath, err)
			return
		}

		log.Info("Formatting %s as %s", vol.FilesystemPath, mountFSType)
		cmd := exec.Command("mke2fs", "-t", mountFSType, vol.FilesystemPath)
		stdOutAndErr, err := cmd.CombinedOutput()
		if err != nil {
			chn <- log.Err("Error formatting %s with %sfilesystem: %s", vol.FilesystemPath, mountFSType, err)
			return
		}
		log.Debug("%s", string(stdOutAndErr))

		// no error
		chn <- nil

		// remove
		if c, ok := driver.volumeMap[req.Name]; ok {
			close(c)
			delete(driver.volumeMap, req.Name)
		}
	}()

	return nil
}

// Remove implementation
func (driver linodeVolumeDriver) Remove(req *volume.RemoveRequest) error {
	//
	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return log.Err("%s", err)
	}

	// Send detach request
	if _, err := driver.linodeAPI.DetachVolume(linVol.ID); err != nil {
		return log.Err("%s", err)
	}

	// Wait for linode to have the volume detached
	if err := waitForLinodeVolumeDetachment(driver.linodeAPI, linVol.ID); err != nil {
		return log.Err("%s", err)
	}

	// Send Delete request
	if err := driver.linodeAPI.DeleteVolume(linVol.ID); err != nil {
		return log.Err("%s", err)
	}
	return nil
}

// Mount implementation
func (driver linodeVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.Info("Called Mount %s", req.Name)

	// wait for volume to finish formatting if it is in queue
	if c, ok := driver.volumeMap[req.Name]; ok {
		log.Info("Waiting for formatting to finish on %s", req.Name)
		err := <-c
		if err != nil {
			return nil, err
		}
	}

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	// attach
	attachOpts := linodego.VolumeAttachOptions{LinodeID: *driver.instanceID}
	if ok, err := driver.linodeAPI.AttachVolume(linVol.ID, &attachOpts); err != nil {
		return nil, log.Err("Error attaching volume to linode: %s", err)
	} else if !ok {
		return nil, log.Err("Could not attach volume to linode.")
	}

	// mkdir
	mp := labelToMountPoint(linVol.Label)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Info("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, log.Err("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	// Wait for linode to have the volume attached
	if err := waitForDeviceFileExists(linVol.FilesystemPath, 180); err != nil {
		return nil, err
	}

	if err := syscall.Mount(linVol.FilesystemPath, mp, mountFSType, syscall.MS_RELATIME, mountOptions); err != nil {
		return nil, log.Err("Error mouting volume(%s) to directory(%s): %s", linVol.FilesystemPath, mp, err)
	}

	log.Info("Mount Call End: %s", req.Name)

	return &volume.MountResponse{Mountpoint: mp}, nil
}

// Path implementation
func (driver linodeVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	log.Info("Path(%s)", req.Name)

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	mp := labelToMountPoint(linVol.Label)
	log.Info("Path(): %s", mp)
	return &volume.PathResponse{Mountpoint: mp}, nil
}

// Unmount implementation
func (driver linodeVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	log.Info("Unmount(%s)", req.Name)

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return err
	}
	if err := syscall.Unmount(labelToMountPoint(linVol.Label), 0); err != nil {
		return log.Err("Unable to GetVolumeByName(%s): %s", req.Name, err)
	}

	log.Info("Unmount(): %s", req.Name)
	return nil
}

// Capabilities implementation
func (driver linodeVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	log.Info("Capabilities(): Scope: global")
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}

// findVolumeByLabel looks up linode volume by label
func (driver linodeVolumeDriver) findVolumeByLabel(volumeLabel string) (*linodego.Volume, error) {
	var jsonFilter []byte
	var err error
	var linVols []*linodego.Volume

	//if jsonFilter, err = json.Marshal(map[string]string{"label": volumeLabel, "region": driver.region}); err != nil {
	if jsonFilter, err = json.Marshal(map[string]string{"label": volumeLabel}); err != nil {
		return nil, err
	}

	listOpts := linodego.NewListOptions(0, string(jsonFilter))
	if linVols, err = driver.linodeAPI.ListInstanceVolumes(*driver.instanceID, listOpts); err != nil {
		return nil, err
	}

	if len(linVols) != 1 {
		return nil, fmt.Errorf("Instance %d Volume with name %s not found", *driver.instanceID, volumeLabel)
	}

	return linVols[0], nil
}

// labelToMountPoint gets the mount-point for a volume
func labelToMountPoint(volumeLabel string) string {
	return path.Join(DefaultMountRoot, volumeLabel)
}

// waitForDeviceFileExists waits until path devicePath becomes available or
// times out.
func waitForDeviceFileExists(devicePath string, waitSeconds int) error {
	return waitForCondition(waitSeconds, 1, func() bool {
		// found, then break
		if _, err := os.Stat(devicePath); !os.IsNotExist(err) {
			return true // condition met
		}
		log.Info("Waiting for device %s to be available", devicePath)
		return false
	})
}

func waitForLinodeVolumeDetachment(linodeAPI linodego.Client, volumeID int) error {
	// Wait for linode to have the volume detached
	return waitForCondition(180, 2, func() bool {
		v, err := linodeAPI.GetVolume(volumeID)
		if err != nil {
			log.Error("%s", err)
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
	return log.Err("waitForCondition timeout")
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
