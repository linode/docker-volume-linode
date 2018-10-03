package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/linode/linodego"
	log "github.com/sirupsen/logrus"
)

type linodeVolumeDriver struct {
	instanceID   int
	region       string
	linodeLabel  string
	linodeToken  string
	mutex        *sync.Mutex
	linodeAPIPtr *linodego.Client
}

// Constructor
func newLinodeVolumeDriver(region string, linodeLabel string, linodeToken string) linodeVolumeDriver {

	driver := linodeVolumeDriver{
		linodeToken: linodeToken,
		region:      region,
		linodeLabel: linodeLabel,
		mutex:       &sync.Mutex{},
	}
	return driver
}

func (driver *linodeVolumeDriver) linodeAPI() (*linodego.Client, error) {

	//
	if driver.linodeToken == "" {
		return nil, fmt.Errorf("Linode Token required.  Set the token by calling \"docker plugin set <plugin-name> linode-token=<linode token>\"")
	}

	if driver.linodeAPIPtr != nil {
		return driver.linodeAPIPtr, nil
	}

	//
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *linodeTokenParamPtr})
	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	api := linodego.NewClient(oauth2Client)
	driver.linodeAPIPtr = &api

	//
	if driver.linodeLabel != "" {
		jsonFilter, _ := json.Marshal(map[string]string{"label": driver.linodeLabel})
		listOpts := linodego.NewListOptions(0, string(jsonFilter))
		linodes, lErr := driver.linodeAPIPtr.ListInstances(context.Background(), listOpts)

		if lErr != nil {
			return nil, fmt.Errorf("Could not determine Linode instance ID from Linode label %s due to error: %s", driver.linodeLabel, lErr)
		} else if len(linodes) != 1 {
			return nil, fmt.Errorf("Could not determine Linode instance ID from Linode label %s", driver.linodeLabel)
		}

		driver.instanceID = linodes[0].ID
	}

	return driver.linodeAPIPtr, nil
}

// Get implementation
func (driver *linodeVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	log.Infof("Get(%s)", req.Name)
	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	if linVol == nil {
		return nil, fmt.Errorf("got a NIL volume. Volume may not exist")
	}

	vol := linodeVolumeToDockerVolume(linVol)
	resp := &volume.GetResponse{Volume: vol}

	log.Infof("Get(): {Name: %s; Mountpoint: %s;}", vol.Name, vol.Mountpoint)

	return resp, nil
}

// List implementation
func (driver *linodeVolumeDriver) List() (*volume.ListResponse, error) {
	log.Infof("List()")

	var jsonFilter []byte
	var err error

	//
	api, err := driver.linodeAPI()
	if err != nil {
		return nil, err
	}

	//
	var volumes []*volume.Volume

	// filters
	if jsonFilter, err = json.Marshal(map[string]string{"region": driver.region}); err != nil {
		return nil, err
	}
	listOpts := linodego.NewListOptions(0, string(jsonFilter))
	log.Debug("linode api listOpts: ", listOpts)

	linVols, err := api.ListVolumes(context.Background(), listOpts)
	if err != nil {
		return nil, err
	}
	log.Debugf("Got %d volume count from api", len(linVols))
	for _, linVol := range linVols {
		vol := linodeVolumeToDockerVolume(linVol)
		log.Debugf("Volume: %+v", vol)
		volumes = append(volumes, vol)
	}
	log.Infof("List() returning %d volumes", len(volumes))
	return &volume.ListResponse{Volumes: volumes}, nil
}

// Create implementation
func (driver *linodeVolumeDriver) Create(req *volume.CreateRequest) error {
	log.Infof("Create(%s)", req.Name)

	api, err := driver.linodeAPI()
	if err != nil {
		return err
	}

	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	var size int
	if sizeOpt, ok := req.Options["size"]; ok {
		s, err := strconv.Atoi(sizeOpt)
		if err != nil {
			return fmt.Errorf("Invalid size")
		}
		size = s
	}

	createOpts := linodego.VolumeCreateOptions{
		Label:    req.Name,
		LinodeID: driver.instanceID,
		Size:     size,
	}

	if _, err := api.CreateVolume(context.Background(), createOpts); err != nil {
		return fmt.Errorf("Create(%s) Failed: %s", req.Name, err)
	}

	return nil
}

// Remove implementation
func (driver *linodeVolumeDriver) Remove(req *volume.RemoveRequest) error {

	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	//
	api, err := driver.linodeAPI()
	if err != nil {
		return err
	}

	//
	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return err
	}

	// Send detach request
	if err := api.DetachVolume(context.Background(), linVol.ID); err != nil {
		return err
	}

	// Wait for linode to have the volume detached
	if err := waitForLinodeVolumeDetachment(*api, linVol.ID); err != nil {
		return err
	}

	// Send Delete request
	if err := api.DeleteVolume(context.Background(), linVol.ID); err != nil {
		return err
	}
	return nil
}

// Mount implementation
func (driver *linodeVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.Infof("Called Mount %s", req.Name)

	api, err := driver.linodeAPI()
	if err != nil {
		return nil, err
	}

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	// If Volume not already attached to this Linode, then attach
	if linVol.LinodeID == nil || *linVol.LinodeID != driver.instanceID {
		// attach
		attachOpts := linodego.VolumeAttachOptions{LinodeID: driver.instanceID}
		if _, err := api.AttachVolume(context.Background(), linVol.ID, &attachOpts); err != nil {
			return nil, fmt.Errorf("Error attaching volume to linode: %s", err)
		}

		if _, err := api.WaitForVolumeLinodeID(context.Background(), linVol.ID, &attachOpts.LinodeID, 180); err != nil {
			return nil, fmt.Errorf("Error attaching volume to linode: %s", err)
		}
	}

	// wait for kernel to have block device available
	if err := waitForDeviceFileExists(linVol.FilesystemPath, 180); err != nil {
		return nil, err
	}

	// Format block device if FS not
	if GetFSType(linVol.FilesystemPath) == "" {
		log.Infof("Formatting device:%s;", linVol.FilesystemPath)
		if err := Format(linVol.FilesystemPath); err != nil {
			return nil, err
		}
	}

	// Create mount point using label (if not exists)
	mp := labelToMountPoint(linVol.Label)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Infof("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, fmt.Errorf("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	if err := Mount(linVol.FilesystemPath, mp); err != nil {
		return nil, fmt.Errorf("Error mouting volume(%s) to directory(%s): %s", linVol.FilesystemPath, mp, err)
	}

	log.Infof("Mount Call End: %s", req.Name)
	return &volume.MountResponse{Mountpoint: mp}, nil
}

// Path implementation
func (driver *linodeVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	log.Infof("Path(%s)", req.Name)

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return nil, err
	}

	mp := labelToMountPoint(linVol.Label)
	log.Infof("Path(): %s", mp)
	return &volume.PathResponse{Mountpoint: mp}, nil
}

// Unmount implementation
func (driver *linodeVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	log.Infof("Unmount(%s)", req.Name)

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return err
	}

	if err := Umount(labelToMountPoint(linVol.Label)); err != nil {
		return fmt.Errorf("Unable to GetVolumeByName(%s): %s", req.Name, err)
	}

	log.Infof("Unmount(): %s", req.Name)
	return nil
}

// Capabilities implementation
func (driver *linodeVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	log.Infof("Capabilities(): Scope: global")
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}

// findVolumeByLabel looks up linode volume by label
func (driver *linodeVolumeDriver) findVolumeByLabel(volumeLabel string) (*linodego.Volume, error) {
	var jsonFilter []byte
	var err error
	var linVols []*linodego.Volume

	//
	api, err := driver.linodeAPI()
	if err != nil {
		return nil, err
	}

	if jsonFilter, err = json.Marshal(map[string]string{"label": volumeLabel, "region": driver.region}); err != nil {
		return nil, err
	}

	listOpts := linodego.NewListOptions(0, string(jsonFilter))
	if linVols, err = api.ListVolumes(context.Background(), listOpts); err != nil {
		return nil, err
	}

	if len(linVols) != 1 {
		return nil, fmt.Errorf("Instance %d Volume with name %s not found", driver.instanceID, volumeLabel)
	}

	return linVols[0], nil
}
