package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/linode/linodego"
	log "github.com/sirupsen/logrus"
)

type linodeVolumeDriver struct {
	instanceID   int
	region       string
	linodeLabel  string
	linodeToken  string
	mountRoot    string
	mutex        *sync.Mutex
	linodeAPIPtr *linodego.Client
}

const (
	fsTagPrefix = "docker-volume-filesystem-"
)

// Constructor
func newLinodeVolumeDriver(linodeLabel, linodeToken, mountRoot string) linodeVolumeDriver {
	driver := linodeVolumeDriver{
		linodeToken: linodeToken,
		linodeLabel: linodeLabel,
		mutex:       &sync.Mutex{},
	}
	if _, err := driver.linodeAPI(); err != nil {
		log.Fatalf("Could not initialize Linode API: %s", err)
	}

	return driver
}

func (driver *linodeVolumeDriver) linodeAPI() (*linodego.Client, error) {
	if driver.linodeToken == "" {
		return nil, fmt.Errorf("Linode Token required.  Set the token by calling \"docker plugin set <plugin-name> linode-token=<linode token>\"")
	}

	if driver.linodeAPIPtr != nil {
		return driver.linodeAPIPtr, nil
	}

	driver.linodeAPIPtr = setupLinodeAPI(driver.linodeToken)

	if driver.instanceID == 0 {
		if err := driver.determineLinodeID(); err != nil {
			driver.linodeAPIPtr = nil
			return nil, err
		}
	}

	return driver.linodeAPIPtr, nil
}

func setupLinodeAPI(token string) *linodego.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	api := linodego.NewClient(oauth2Client)
	ua := fmt.Sprintf("docker-volume-linode/%s linodego/%s", VERSION, linodego.Version)
	api.SetUserAgent(ua)
	return &api
}

func (driver *linodeVolumeDriver) determineLinodeID() error {
	if driver.linodeLabel == "" {
		var hostnameErr error
		driver.linodeLabel, hostnameErr = os.Hostname()
		if hostnameErr != nil {
			return fmt.Errorf("Could not determine hostname: %s", hostnameErr)
		}
	}

	jsonFilter, _ := json.Marshal(map[string]string{"label": driver.linodeLabel})
	listOpts := linodego.NewListOptions(0, string(jsonFilter))
	linodes, lErr := driver.linodeAPIPtr.ListInstances(context.Background(), listOpts)

	if lErr != nil {
		return fmt.Errorf("Could not determine Linode instance ID from Linode label %s due to error: %s", driver.linodeLabel, lErr)
	} else if len(linodes) != 1 {
		return fmt.Errorf("Could not determine Linode instance ID from Linode label %s", driver.linodeLabel)
	}

	driver.instanceID = linodes[0].ID
	if driver.region == "" {
		driver.region = linodes[0].Region
	}
	return nil
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

	mp := driver.labelToMountPoint(linVol.Label)
	vol := linodeVolumeToDockerVolume(*linVol, mp)
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
		mp := driver.labelToMountPoint(linVol.Label)
		vol := linodeVolumeToDockerVolume(linVol, mp)
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

	if fsOpt, ok := req.Options["filesystem"]; ok {
		createOpts.Tags = append(createOpts.Tags, fsTagPrefix+fsOpt)
	}

	if deleteOpt, ok := req.Options["delete-on-remove"]; ok {
		b, err := strconv.ParseBool(deleteOpt)
		if err != nil {
			return fmt.Errorf("Invalid delete-on-remove argument")
		}
		if b {
			createOpts.Tags = append(createOpts.Tags, "docker-volume-delete-on-remove")
		}
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
	if err := detachAndWait(api, linVol.ID); err != nil {
		return err
	}

	// Optionally send Delete request
	for _, t := range linVol.Tags {
		if t == "docker-volume-delete-on-remove" {
			if err := api.DeleteVolume(context.Background(), linVol.ID); err != nil {
				return err
			}
			break
		}
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

	linVol, err = api.GetVolume(context.Background(), linVol.ID)
	if err != nil {
		return nil, err
	}

	// Ensure the volume is not currently mounted
	if err := driver.ensureNotMounted(linVol.ID); err != nil {
		return nil, fmt.Errorf("failed to attach volume: %s", err)
	}

	// Attach to current Linode
	if err := attachAndWait(api, linVol.ID, driver.instanceID); err != nil {
		return nil, fmt.Errorf("error attaching volume(%s) to linode: %s", req.Name, err)
	}

	// wait for kernel to have block device available
	if err := waitForDeviceFileExists(linVol.FilesystemPath, 300); err != nil {
		return nil, err
	}

	// Format block device if no FS found
	if GetFSType(linVol.FilesystemPath) == "" {
		log.Infof("Formatting device:%s;", linVol.FilesystemPath)
		filesystem := "ext4"
		for _, tag := range linVol.Tags {
			if strings.HasPrefix(tag, fsTagPrefix) {
				filesystem = tag[len(fsTagPrefix):]
				break
			}
		}
		if err := Format(linVol.FilesystemPath, filesystem); err != nil {
			return nil, err
		}
	}

	// Create mount point using label (if not exists)
	mp := driver.labelToMountPoint(linVol.Label)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		log.Infof("Creating mountpoint directory: %s", mp)
		if err = os.MkdirAll(mp, 0755); err != nil {
			return nil, fmt.Errorf("Error creating mountpoint directory(%s): %s", mp, err)
		}
	}

	if err := Mount(linVol.FilesystemPath, mp); err != nil {
		return nil, fmt.Errorf("Error mounting volume(%s) to directory(%s): %s", linVol.FilesystemPath, mp, err)
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

	mp := driver.labelToMountPoint(linVol.Label)
	log.Infof("Path(): %s", mp)
	return &volume.PathResponse{Mountpoint: mp}, nil
}

// Unmount implementation
func (driver *linodeVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	api, err := driver.linodeAPI()
	if err != nil {
		return err
	}

	log.Infof("Unmount(%s)", req.Name)

	linVol, err := driver.findVolumeByLabel(req.Name)
	if err != nil {
		return err
	}

	if err := Umount(driver.labelToMountPoint(linVol.Label)); err != nil {
		return fmt.Errorf("Unable to Unmount(%s): %s", req.Name, err)
	}

	log.Infof("Unmount(): %s", req.Name)

	// The volume is detached from the Linode at unmount
	// to allow remote Linodes to infer whether a volume is
	// mounted
	if err := detachAndWait(api, linVol.ID); err != nil {
		return err
	}

	return nil
}

// Capabilities implementation
func (driver *linodeVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	log.Infof("Capabilities(): Scope: global")
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}

// labelToMountPoint gets the mount-point for a volume
func (driver *linodeVolumeDriver) labelToMountPoint(volumeLabel string) string {
	return path.Join(driver.mountRoot, volumeLabel)
}

// findVolumeByLabel looks up linode volume by label
func (driver *linodeVolumeDriver) findVolumeByLabel(volumeLabel string) (*linodego.Volume, error) {
	var jsonFilter []byte
	var err error
	var linVols []linodego.Volume

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

	return &linVols[0], nil
}

func detachAndWait(api *linodego.Client, volumeID int) error {
	// Send detach request
	if err := api.DetachVolume(context.Background(), volumeID); err != nil {
		return fmt.Errorf("Error detaching volumeID(%d): %s", volumeID, err)
	}

	// Wait for linode to have the volume detached
	if err := waitForLinodeVolumeDetachment(*api, volumeID, 180); err != nil {
		return fmt.Errorf("Error waiting for detachment of volumeID(%d): %s", volumeID, err)
	}
	return nil
}

func attachAndWait(api *linodego.Client, volumeID int, linodeID int) error {
	// attach
	attachOpts := linodego.VolumeAttachOptions{LinodeID: linodeID}
	if _, err := api.AttachVolume(context.Background(), volumeID, &attachOpts); err != nil {
		return fmt.Errorf("Error attaching volume(%d) to linode(%d): %s", volumeID, linodeID, err)
	}

	if _, err := api.WaitForVolumeLinodeID(context.Background(), volumeID, &linodeID, 300); err != nil {
		return fmt.Errorf("Error waiting for attachment of volume(%d) to linode(%d): %s", volumeID, linodeID, err)
	}
	return nil
}

// ensureNotMounted returns an error if the specified volume is in use by a remote Linode
// or local container
func (driver *linodeVolumeDriver) ensureNotMounted(volumeID int) error {
	api, err := driver.linodeAPI()
	if err != nil {
		return err
	}

	vol, err := api.GetVolume(context.Background(), volumeID)
	if err != nil {
		return err
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	f := filters.NewArgs()
	f.Add("volume", vol.Label)
	listOpts := types.ContainerListOptions{
		Filters: f,
	}

	containers, err := cli.ContainerList(context.Background(), listOpts)
	if err != nil {
		return fmt.Errorf("failed to list containers with specified volume: %v", err)
	}

	if len(containers) > 0 {
		return fmt.Errorf("volume in use by %s", containers[0].ID)
	}

	// We should wait for the volume to be detached if it is in the process of detaching
	if vol.LinodeID != nil {
		volDetaching, err := checkVolumeDetaching(api, volumeID)
		if err != nil {
			return err
		}

		if !volDetaching {
			return fmt.Errorf("volume is currently in use by linode id %d", *vol.LinodeID)
		}

		if err := waitForLinodeVolumeDetachment(*api, volumeID, 180); err != nil {
			return err
		}
	}

	return nil
}

// checkVolumeDetaching checks whether a volume is currently in the process of detaching.
// This is useful cases where a volume is available but hasn't yet been fully attached
func checkVolumeDetaching(api *linodego.Client, volumeID int) (bool, error) {
	filter := linodego.Filter{}

	filter.AddField(linodego.Eq, "entity.id", volumeID)
	filter.AddField(linodego.Eq, "entity.type", "volume")
	filter.OrderBy = "created"
	filter.Order = "desc"

	detachFilterStr, err := filter.MarshalJSON()

	if err != nil {
		return false, err
	}

	events, err := api.ListEvents(context.Background(),
		&linodego.ListOptions{Filter: string(detachFilterStr)})
	if err != nil {
		return false, err
	}

	for _, e := range events {
		if e.Action == "volume_detach" {
			return true, nil
		}

		if e.Action == "volume_attach" {
			return false, nil
		}
	}

	return false, nil
}