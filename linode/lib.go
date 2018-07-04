package linode

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/libgolang/log"
	"gopkg.in/resty.v1"
)

// API interface representing hight level
// operations using Linode API
type API interface {
	//VolumeList() []Volume
	GetVolumeByName(name string) *Volume
	Attach(volumeName string) error
	EachVolume(callBack func(Volume) bool) error
	CreateVolume(name string, m map[string]string) (*Volume, error)
	Detach(volumeName string) error
	RemoveVolume(volumeName string) error
}

// API implementation
type api struct {
	token  string
	region string
	host   string
}

// NewAPI new API instance
func NewAPI(token, region, host string) API {
	return &api{token, region, host}
}

// CreateVolume creates a linode volume
func (a *api) CreateVolume(name string, m map[string]string) (*Volume, error) {

	// Size
	size := 20
	if sizeStr, ok := m["size"]; ok {
		if sizeTmp, err := strconv.Atoi(sizeStr); err == nil {
			size = sizeTmp
		} else {
			return nil, log.Err("Unable to parse volume size `%s`", sizeTmp)
		}
	}

	// Request
	req := &Volume{
		Label:  name,
		Size:   size,
		Region: a.region,
	}

	volItf, err := a.Post("https://api.linode.com/v4/volumes", req, &Volume{})
	if err != nil {
		return nil, log.Err("Error requesting colume creation: %s", err)
	}

	vol, ok := volItf.(*Volume)
	if !ok {
		return nil, log.Err("Unable to cast response from server when creating volume: %s", name)
	}

	return vol, nil
}

func (a *api) Detach(volumeName string) error {
	volumeID, err := a.getVolumeIDByName(volumeName)
	if err != nil {
		return log.Err("Unable to get Volume ID by name(%s)", volumeName)
	}

	if err := a.detachByVolumeID(volumeID); err != nil {
		return err
	}
	return nil
}

func (a *api) RemoveVolume(volumeName string) error {
	volumeID, err := a.getVolumeIDByName(volumeName)
	if err != nil {
		return log.Err("Unable to get Volume ID by name(%s)", volumeName)
	}

	url := fmt.Sprintf("https://api.linode.com/v4/volumes/%d", volumeID)
	if _, err := a.DELETE(url, nil); err != nil {
		return err
	}
	return nil
}

// Attach attaches a volume to a linode
func (a *api) Attach(volumeName string) error {
	linodeID, err := a.getLinodeIDByName(a.host)
	if err != nil {
		return log.Err("Unable to get Linode ID by name(%s): %s", a.host, err)
	}

	vol := a.GetVolumeByName(volumeName)
	if vol == nil {
		return log.Err("Unable to find Volume name(%s)", volumeName)
	}

	if err := a.detachByVolumeID(*vol.ID); err != nil {
		return err
	}

	// attach
	log.Info("Calling attach on volume %d and node %d", *vol.ID, linodeID)
	url := fmt.Sprintf("https://api.linode.com/v4/volumes/%d/attach", *vol.ID)
	body := AttachRequest{LinodeID: &linodeID}
	if _, err := a.Post(url, body, nil); err != nil {
		return log.Err("unable to attach volume: %s", err)
	}

	// wait for device to become available
	for i := 0; i < 60; i++ {
		if _, err := os.Stat(*vol.FilesystemPath); !os.IsNotExist(err) {
			break
		}
		log.Info("Waiting for kernel to attach %s", *vol.FilesystemPath)
		time.Sleep(2 * time.Second) // sleep 2 seconds
	}
	if _, err := os.Stat(*vol.FilesystemPath); os.IsNotExist(err) {
		return log.Err("Attached volume device(%s) not found", *vol.FilesystemPath)
	}

	return nil
}

func (a *api) detachByVolumeID(volumeID int) error {
	// detach
	log.Info("Calling detach on volume %d", volumeID)
	detachURL := fmt.Sprintf("https://api.linode.com/v4/volumes/%d/detach", volumeID)
	if _, err := a.Post(detachURL, nil, nil); err != nil {
		return log.Err("Detaching request returned error: %s", err)
	}

	// wait for deatch request to finish
	for i := 0; i < 60; i++ {
		it, err := a.Get(fmt.Sprintf("https://api.linode.com/v4/volumes/%d", volumeID), &Volume{})
		if err != nil {
			log.Warn("Detach Wait request failed")
		}
		vol, ok := it.(*Volume)
		if ok {
			if vol.LinodeID == nil || *vol.LinodeID == 0 {
				return nil // happy path
			}
		}

		log.Info("Waiting for linode to detach volume(%d)", volumeID)
		time.Sleep(2 * time.Second) // sleep 2 seconds
	}

	return log.Err("Detaching volumeID %d failed. Timed out!", volumeID)
}

// getLinodeIDByName resturns the id of the linode given the name or returns empty
// string if not found
func (a *api) getLinodeIDByName(linodeName string) (int, error) {
	pages := 1
	for page := 1; page <= pages; page++ {
		url := fmt.Sprintf("https://api.linode.com/v4/linode/instances?page=%d", page)
		//var err error
		it, err := a.Get(url, &ListNodeResponse{})
		if err != nil {
			return 0, err
		}
		resp, ok := it.(*ListNodeResponse)
		if !ok {
			return 0, fmt.Errorf("Error casting to ListNodeReponse")
		}
		pages = resp.Pages
		for _, n := range resp.Data {
			if n.Label == linodeName {
				return n.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("Not Found")
}

// EachVolume iterate through all volumes
func (a *api) EachVolume(callBack func(Volume) bool) error {
	pages := 1
	for page := 1; page <= pages; page++ {
		url := fmt.Sprintf("https://api.linode.com/v4/volumes?page=%d", page)
		it, err := a.Get(url, &ListVolumeResponse{})
		if err != nil {
			return err
		}
		resp, ok := it.(*ListVolumeResponse)
		if !ok {
			return fmt.Errorf("Error casting to ListVolumeReponse")
		}
		pages = resp.Pages
		for _, n := range resp.Data {
			if n.Region != a.region {
				continue
			}
			if !callBack(n) {
				break
			}
		}
	}
	return nil
}

// GetVolumeByName returns the volume with the given name or nil if not found
func (a *api) GetVolumeByName(name string) *Volume {
	var res *Volume
	_ = a.EachVolume(func(v Volume) bool {
		if v.Label == name {
			res = &v
			return false
		}
		return true
	})
	return res
}

func (a *api) getVolumeIDByName(volumeName string) (int, error) {
	pages := 1
	for page := 1; page <= pages; page++ {
		url := fmt.Sprintf("https://api.linode.com/v4/volumes?page=%d", page)
		it, err := a.Get(url, &ListVolumeResponse{})
		if err != nil {
			return 0, err
		}
		resp, ok := it.(*ListVolumeResponse)
		if !ok {
			return 0, fmt.Errorf("Error casting to ListVolumeReponse")
		}
		pages = resp.Pages
		for _, n := range resp.Data {
			if n.Label == volumeName {
				return *n.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("Not Found")
}

// Get REST GET request
func (a *api) Get(url string, res interface{}) (interface{}, error) {
	log.Debug("GET %s token: %s", url, a.token)
	r := resty.R()
	if res != nil {
		r.SetResult(res)
	}
	r.SetHeader("Authorization", fmt.Sprintf("Bearer %s", a.token))
	resp, err := r.Get(url)
	if err == nil && resp.StatusCode() != 200 {
		return nil, fmt.Errorf("GET Request returned error %d: ", resp.StatusCode())
	}
	return resp.Result(), err
}

// Post REST POST request
func (a *api) Post(url string, req interface{}, res interface{}) (interface{}, error) {
	log.Debug("POST %s", url)

	r := resty.R()
	if req != nil {
		log.Debug("%+v", req)
		r.SetBody(req)
	}

	if res != nil {
		r.SetResult(res)
	}

	r.SetHeader("Authorization", fmt.Sprintf("Bearer %s", a.token))
	resp, err := r.Post(url)
	if err == nil && resp.StatusCode() != 200 {
		bytes := resp.Body()
		return nil, fmt.Errorf("POST %s returned error [%d]: --- %s", url, resp.StatusCode(), string(bytes))
	}
	return resp.Result(), err
}

// DELETE REST DELETE request
func (a *api) DELETE(url string, res interface{}) (interface{}, error) {
	log.Debug("DELETE %s token: %s", url, a.token)
	r := resty.R()
	if res != nil {
		r.SetResult(res)
	}
	r.SetHeader("Authorization", fmt.Sprintf("Bearer %s", a.token))
	resp, err := r.Delete(url)
	if err == nil && resp.StatusCode() != 200 {
		return nil, fmt.Errorf("GET Request returned error %d: ", resp.StatusCode())
	}
	return resp.Result(), err
}

// ListNodeResponse list node response
type ListNodeResponse struct {
	Data    []Node `json:"data"`
	Page    int    `json:"page"`    // "page": 1,
	Pages   int    `json:"pages"`   // "pages": 1,
	Results int    `json:"results"` // "results": 1
}

// ListVolumeResponse list volume response
type ListVolumeResponse struct {
	Data    []Volume `json:"data"`
	Page    int      `json:"page"`    // "page": 1,
	Pages   int      `json:"pages"`   // "pages": 1,
	Results int      `json:"results"` // "results": 1
}

// Node node
type Node struct {
	ID     int    `json:"id"`     //"id": 123,
	Label  string `json:"label"`  //"label": "linode123",
	Region string `json:"region"` //"region": "us-east",
	//"image": "linode/debian9",
	//"type": "g6-standard-2",
	//"group": "Linode-Group",
	//"status": "running",
	//"hypervisor": "kvm",
	//"created": "2018-01-01T00:01:01",
	//"updated": "2018-01-01T00:01:01",
	//...
	//...
	//...
}

// Volume volume
type Volume struct {
	ID             *int    `json:"id"`              // "id": 12345,
	Label          string  `json:"label"`           // "label": "my-volume",
	FilesystemPath *string `json:"filesystem_path"` // "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
	LinodeID       *int    `json:"linode_id"`       // "linode_id": 12346,
	Region         string  `json:"region"`          // "region": "us-east",
	Size           int     `json:"size"`
	// "status": "active",
	// "created": "2018-01-01T00:01:01",
	// "updated": "2018-01-01T00:01:01"
}

// AttachRequest linode Attach Request
type AttachRequest struct {
	LinodeID *int    `json:"linode_id"`
	ConfigID *string `json:"config_id"`
}

func getHostName() string {
	h, _ := os.Hostname()
	return h
}

// Mountpoint returns the mountpoint
func (v *Volume) Mountpoint() string {
	return path.Join("/mnt", v.Label)
}
