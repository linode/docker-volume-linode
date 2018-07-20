package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/chiefy/linodego"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/log"
)

type linodeVolumeDriver struct {
	linodeAPI   linodego.Client
	instanceID  *int
	region      string
	linodeLabel *string
	mutex       *sync.Mutex
}

// Constructor
func newLinodeVolumeDriver(linodeAPI linodego.Client, region string, linodeLabel *string) linodeVolumeDriver {
	driver := linodeVolumeDriver{
		linodeAPI:   linodeAPI,
		region:      region,
		linodeLabel: linodeLabel,
		mutex:       &sync.Mutex{},
	}

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
	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	var size int
	if sizeOpt, ok := req.Options["size"]; ok {
		s, err := strconv.Atoi(sizeOpt)
		if err != nil {
			return log.Err("Invalid size")
		}
		size = s
	}

	createOpts := linodego.VolumeCreateOptions{
		Label:    req.Name,
		LinodeID: *driver.instanceID,
		Size:     size,
	}
	if _, err := driver.linodeAPI.CreateVolume(createOpts); err != nil {
		return log.Err("Create(%s) Failed: %s", req.Name, err)
	}

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

	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	// If Volume not already attached to this Linode, then attach
	if linVol.LinodeID == nil || *linVol.LinodeID != *driver.instanceID {
		// attach
		attachOpts := linodego.VolumeAttachOptions{LinodeID: *driver.instanceID}
		if ok, err := driver.linodeAPI.AttachVolume(linVol.ID, &attachOpts); err != nil {
			return nil, log.Err("Error attaching volume to linode: %s", err)
		} else if !ok {
			return nil, log.Err("Could not attach volume to linode.")
		}
		if err := linodego.WaitForVolumeLinodeID(&driver.linodeAPI, linVol.ID, &attachOpts.LinodeID, 180); err != nil {
			return nil, log.Err("Error attaching volume to linode: %s", err)
		}
	}

	// wait for kernel to have block device available
	if err := waitForDeviceFileExists(linVol.FilesystemPath, 180); err != nil {
		return nil, err
	}

	// Format block device if FS not
	if GetFSType(linVol.FilesystemPath) == "" {
		log.Info("Formatting device:%s;", linVol.FilesystemPath)
		if err := Format(linVol.FilesystemPath); err != nil {
			return nil, err
		}
	}

	// Create mount point using label (if not exists)
	mp := labelToMountPoint(linVol.Label)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Info("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, log.Err("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	if err := Mount(linVol.FilesystemPath, mp); err != nil {
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

	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return err
	}

	if err := Umount(labelToMountPoint(linVol.Label)); err != nil {
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
