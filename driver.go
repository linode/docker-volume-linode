package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/docker-volume-linode/linode"
	"github.com/libgolang/log"
)

type linodeVolumeDriver struct {
	linodeAPI linode.API
}

// Constructor
func newLinodeVolumeDriver(linodeAPI linode.API) linodeVolumeDriver {
	return linodeVolumeDriver{linodeAPI: linodeAPI}
}

// Get implementation
func (driver linodeVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	log.Info("Get(%s)", req.Name)
	linVol := driver.linodeAPI.GetVolumeByName(req.Name)
	if linVol != nil {
		log.Info("Get(): req.Name")
		vol := &volume.Volume{
			Name:       linVol.Label,
			Mountpoint: linVol.Mountpoint(),
		}
		return &volume.GetResponse{Volume: vol}, nil
	}
	log.Warn("Volume with name %s not found")
	return nil, fmt.Errorf("Volume with name %s not found", req.Name)
}

// List implementation
func (driver linodeVolumeDriver) List() (*volume.ListResponse, error) {
	log.Info("List()")

	//
	var volumes []*volume.Volume
	err := driver.linodeAPI.EachVolume(func(linVol linode.Volume) bool {
		vol := &volume.Volume{
			Name:       linVol.Label,
			Mountpoint: linVol.Mountpoint(),
		}
		volumes = append(volumes, vol)
		return true
	})

	log.Info("List(): %s", volumes)
	return &volume.ListResponse{Volumes: volumes}, err
}

// Create implementation
func (driver linodeVolumeDriver) Create(req *volume.CreateRequest) error {
	log.Info("Create(%s)", req.Name)

	vol, err := driver.linodeAPI.CreateVolume(req.Name, req.Options)
	if err != nil {
		return log.Err("Create(%s) Failed: %s", req.Name, err)
	}

	if err = driver.linodeAPI.Attach(req.Name); err != nil {
		return log.Err("Create(%s) Failed: %s", req.Name, err)
	}

	// format drive
	log.Info("Creating ext4 filesystem on %s", *vol.FilesystemPath)
	cmd := exec.Command("mke2fs", "-t", "ext4", *vol.FilesystemPath)
	stdOutAndErr, err := cmd.CombinedOutput()
	if err != nil {
		return log.Err("Error formatting %s with ext4 filesystem: %s", *vol.FilesystemPath, err)
	}
	log.Debug("%s", string(stdOutAndErr))
	return nil
}

// Remove implementation
func (driver linodeVolumeDriver) Remove(req *volume.RemoveRequest) error {
	if err := driver.linodeAPI.Detach(req.Name); err != nil {
		return err
	}
	if err := driver.linodeAPI.RemoveVolume(req.Name); err != nil {
		return err
	}
	return nil
}

// Mount implementation
func (driver linodeVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.Info("Called Mount %s", req.Name)

	vol := driver.linodeAPI.GetVolumeByName(req.Name)
	if vol == nil {
		return nil, log.Err("Volume not found")
	}

	// attach
	if err := driver.linodeAPI.Attach(vol.Label); err != nil {
		return nil, log.Err("Error attaching volume to linode: %s", err)
	}

	// mkdir
	mp := vol.Mountpoint()
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Info("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, log.Err("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	// Wait for linode to have the volumen attached
	for i := 0; i < 10; i++ {
		// found, then break
		if _, err := os.Stat(*vol.FilesystemPath); !os.IsNotExist(err) {
			break
		}
		log.Info("Waiting for linode to attach %s", *vol.FilesystemPath)
		time.Sleep(time.Second * 2)
	}
	if err := syscall.Mount(*vol.FilesystemPath, mp, "ext4", syscall.MS_RELATIME, "data=ordered"); err != nil {
		return nil, log.Err("Error mouting volume(%s) to directory(%s): %s", *vol.FilesystemPath, mp, err)
	}

	log.Info("Mount Call End: %s", req.Name)

	return &volume.MountResponse{Mountpoint: mp}, nil
}

// Path implementation
func (driver linodeVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	log.Info("Path(%s)", req.Name)

	vol := driver.linodeAPI.GetVolumeByName(req.Name)
	if vol == nil {
		return nil, log.Err("Volume %s not found", req.Name)
	}

	mp := vol.Mountpoint()
	log.Info("Path(): %s", mp)
	return &volume.PathResponse{Mountpoint: mp}, nil
}

// Unmount implementation
func (driver linodeVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	log.Info("Unmount(%s)", req.Name)

	//
	vol := driver.linodeAPI.GetVolumeByName(req.Name)
	if err := syscall.Unmount(vol.Mountpoint(), 0); err != nil {
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
