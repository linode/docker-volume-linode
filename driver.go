package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

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
		mountRoot:   mountRoot,
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
		// If the label isn't defined, we should determine the IP through the network interface
		log.Infof("Using network interface to determine Linode ID")

		if err := driver.determineLinodeIDFromNetworking(); err != nil {
			return fmt.Errorf("Failed to determine Linode ID from networking: %s\n"+
				"If this error continues to occur or if you are using a custom network configuration, "+
				"consider using the `linode-label` flag.", err)
		}

		return nil
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

func (driver *linodeVolumeDriver) resolveMachineLinkLocal() (string, error) {
	// We only want to filter on eth0 for Link Local.
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ifa, ok := addr.(*net.IPNet); ok {
			if ifa.IP.To4() != nil {
				continue
			}

			if !ifa.IP.IsLinkLocalUnicast() {
				continue
			}
			return strings.Split(addr.String(), "/")[0], nil
		}
	}

	return "", fmt.Errorf("no link local ipv6 address found")
}

func (driver *linodeVolumeDriver) determineLinodeIDFromNetworking() error {
	linkLocal, err := driver.resolveMachineLinkLocal()
	if err != nil {
		return fmt.Errorf("failed to determine linode id from networking: %s", err)
	}

	instances, err := driver.linodeAPIPtr.ListInstances(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to list instances: %s", err)
	}

	for _, instance := range instances {
		ips, err := driver.linodeAPIPtr.GetInstanceIPAddresses(context.Background(), instance.ID)
		if err != nil {
			return fmt.Errorf("failed to get ip addresses for instance %d: %s", instance.ID, err)
		}

		if ips.IPv6.LinkLocal == nil || ips.IPv6.LinkLocal.Address != linkLocal {
			continue
		}

		driver.instanceID = instance.ID
		driver.region = instance.Region
		return nil
	}

	return fmt.Errorf("instance with link local address %s not found", linkLocal)
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
		Label:  req.Name,
		Region: driver.region,
		Size:   size,
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
	if err := driver.ensureVolumeAttached(linVol.ID); err != nil {
		return nil, fmt.Errorf("failed to attach volume: %s", err)
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

// ensureVolumeAttached attempts to attach a volume to the current Linode instance
func (driver *linodeVolumeDriver) ensureVolumeAttached(volumeID int) error {
	// TODO: validate whether a volume is in use in a local container

	api, err := driver.linodeAPI()
	if err != nil {
		return err
	}

	// Wait for detachment if already detaching
	if err := waitForVolumeNotBusy(api, volumeID); err != nil {
		return err
	}

	// Fetch volume
	vol, err := api.GetVolume(context.Background(), volumeID)
	if err != nil {
		return err
	}

	// If volume is already attached, do nothing
	if vol.LinodeID != nil && *vol.LinodeID == driver.instanceID {
		return nil
	}

	// Forcibly attach the volume if forceAttach is enabled
	if forceAttach && vol.LinodeID != nil && *vol.LinodeID != driver.instanceID {
		if err := detachAndWait(api, volumeID); err != nil {
			return err
		}

		return attachAndWait(api, volumeID, driver.instanceID)
	}

	// Throw an error if the instance is not in an attachable state
	if vol.LinodeID != nil && *vol.LinodeID != driver.instanceID {
		return fmt.Errorf("failed to attach volume: volume is currently attached to linode %d", *vol.LinodeID)
	}

	return attachAndWait(api, volumeID, driver.instanceID)
}

// waitForVolumeNotBusy checks whether a volume is currently busy.
func waitForVolumeNotBusy(api *linodego.Client, volumeID int) error {
	vol, err := api.GetVolume(context.Background(), volumeID)
	if err != nil {
		return err
	}

	if vol.LinodeID == nil {
		return nil
	}

	filter := linodego.Filter{}

	filter.AddField(linodego.Eq, "entity.id", volumeID)
	filter.AddField(linodego.Eq, "entity.type", "volume")
	filter.OrderBy = "created"
	filter.Order = "desc"

	detachFilterStr, err := filter.MarshalJSON()

	if err != nil {
		return err
	}

	events, err := api.ListEvents(context.Background(),
		&linodego.ListOptions{Filter: string(detachFilterStr)})
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.Status != "started" {
			continue
		}

		if err := waitForEventFinished(api, event.ID); err != nil {
			return err
		}
	}

	return nil
}

func waitForEventFinished(api *linodego.Client, eventID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(60)*time.Second)
	defer cancel()

	ticker := time.NewTicker(2000 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			event, err := api.GetEvent(ctx, eventID)
			if err != nil {
				return err
			}

			if event.Status == "finished" || event.Status == "failed" {
				return nil
			}

		case <-ctx.Done():
			return fmt.Errorf("error waiting for event(%d) completion: %v", eventID, ctx.Err())
		}
	}
}
