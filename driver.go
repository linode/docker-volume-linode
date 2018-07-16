package main

import (
	"encoding/json"
	"fmt"
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
}

// Constructor
func newLinodeVolumeDriver(linodeAPI linodego.Client, region string, linodeLabel *string) linodeVolumeDriver {
	driver := linodeVolumeDriver{linodeAPI: linodeAPI, region: region, linodeLabel: linodeLabel}

	if linodeLabel != nil {
		jsonFilter, _ := json.Marshal(map[string]string{"label": *linodeLabel})
		listOpts := linodego.NewListOptions(0, string(jsonFilter))
		linodes, lErr := driver.linodeAPI.ListInstances(listOpts)

		if lErr != nil {
			log.Err("Could not determine Linode instance ID from Linode label %s due to error: %s", *linodeLabel, lErr)
			os.Exit(1)
		} else if len(linodes) != 1 {
			log.Err("Could not determine Linode instance ID from Linode label %s", *linodeLabel)
			os.Exit(1)
		}

		driver.instanceID = &linodes[0].ID
	}

	// @TODO what is the plan for this driver if we are running without being tied to a specific Linode?

	return driver
}

func (driver linodeVolumeDriver) volume(volumeLabel string) (linVol *linodego.Volume, err error) {
	jsonFilter, _ := json.Marshal(map[string]string{"label": volumeLabel})
	listOpts := linodego.NewListOptions(0, string(jsonFilter))
	linVols, lErr := driver.linodeAPI.ListInstanceVolumes(*driver.instanceID, listOpts)
	if lErr != nil {
		err = nil
	} else if len(linVols) != 1 {
		err = fmt.Errorf("Instance %d Volume with name %s not found", driver.instanceID, linVol)
	} else {
		linVol = linVols[0]
	}
	return linVol, err
}

// Get implementation
func (driver linodeVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	log.Info("Get(%s)", req.Name)
	linVol, err := driver.volume(req.Name)
	if err != nil {
		vol := &volume.Volume{
			Name:       linVol.Label,
			Mountpoint: mountPoint(linVol.Label),
		}
		return &volume.GetResponse{Volume: vol}, nil
	}
	log.Warn(err.Error())
	return nil, err
}

// List implementation
func (driver linodeVolumeDriver) List() (*volume.ListResponse, error) {
	log.Info("List()")

	//
	var volumes []*volume.Volume

	linVols, err := driver.linodeAPI.ListInstanceVolumes(*driver.instanceID, nil)

	if err != nil {
		for _, linVol := range linVols {
			vol := &volume.Volume{
				Name:       linVol.Label,
				Mountpoint: mountPoint(linVol.Label),
			}
			volumes = append(volumes, vol)
		}
	}

	log.Info("List(): %s", volumes)
	return &volume.ListResponse{Volumes: volumes}, err
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

	linodego.WaitForVolumeStatus(&driver.linodeAPI, vol.ID, linodego.VolumeActive, 180)

	// format drive
	log.Info("Creating %s filesystem on %s", mountFSType, vol.FilesystemPath)
	cmd := exec.Command("mke2fs", "-t", mountFSType, vol.FilesystemPath)
	stdOutAndErr, err := cmd.CombinedOutput()
	if err != nil {
		return log.Err("Error formatting %s with %sfilesystem: %s", vol.FilesystemPath, mountFSType, err)
	}
	log.Debug("%s", string(stdOutAndErr))
	return nil
}

// Remove implementation
func (driver linodeVolumeDriver) Remove(req *volume.RemoveRequest) error {
	linVol, err := driver.volume(req.Name)
	if err != nil {
		return err
	}
	if _, err := driver.linodeAPI.DetachVolume(linVol.ID); err != nil {
		return err
	}
	// @TODO what should we do if we can't detach?
	if err := driver.linodeAPI.DeleteVolume(linVol.ID); err != nil {
		return err
	}
	return nil
}

// Mount implementation
func (driver linodeVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.Info("Called Mount %s", req.Name)

	linVol, err := driver.volume(req.Name)
	if err != nil {
		return nil, err
	}

	// attach
	attachOpts := linodego.VolumeAttachOptions{
		LinodeID: *driver.instanceID,
	}
	if ok, err := driver.linodeAPI.AttachVolume(linVol.ID, &attachOpts); err != nil {
		return nil, log.Err("Error attaching volume to linode: %s", err)
	} else if !ok {
		return nil, log.Err("Could not attach volume to linode.")
	}

	// mkdir
	mp := mountPoint(linVol.Label)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Info("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, log.Err("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	// Wait for linode to have the volume attached
	for i := 0; i < 10; i++ {
		// found, then break
		if _, err := os.Stat(linVol.FilesystemPath); !os.IsNotExist(err) {
			break
		}
		log.Info("Waiting for linode to attach %s", linVol.FilesystemPath)
		time.Sleep(time.Second * 2)
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

	linVol, err := driver.volume(req.Name)
	if err != nil {
		return nil, err
	}

	mp := mountPoint(linVol.Label)
	log.Info("Path(): %s", mp)
	return &volume.PathResponse{Mountpoint: mp}, nil
}

// Unmount implementation
func (driver linodeVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	log.Info("Unmount(%s)", req.Name)

	linVol, err := driver.volume(req.Name)
	if err != nil {
		return err
	}
	if err := syscall.Unmount(mountPoint(linVol.Label), 0); err != nil {
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

// mountPoint gets the mount-point for a volume
func mountPoint(volumeLabel string) string {
	return path.Join(DefaultMountRoot, volumeLabel)
}
