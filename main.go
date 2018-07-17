package main

import (
	"os"

	"github.com/chiefy/linodego"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/config"
	"github.com/libgolang/log"
)

const (
	// DefaultSocketAddress docket file to be created for communication with docker
	DefaultSocketAddress = "/run/docker/plugins/linode-driver.sock"
	// DefaultSocketGID Group ownership of DefaultSocketAddress
	DefaultSocketGID = 0
	// DefaultMountRoot Directory to use for mounting Linode Volume Devices
	DefaultMountRoot = "/mnt/"
)

var (
	logLevelParamPtr      = config.String("log-level", "DEBUG", "Log Level. Defaults to WARN")
	logTraceParamPtr      = config.Bool("log-trace", true, "Set Tracing to true")
	socketAddressParamPtr = config.String("socket-file", DefaultSocketAddress, "Sets the socket file/address.")
	socketGIDParamPtr     = config.Int("socket-gid", DefaultSocketGID, "Sets the socket group id.")
	mountRootParamPtr     = config.String("mount-root", DefaultMountRoot, "Sets the root directory for volume mounts.")
	linodeTokenParamPtr   = config.String("linode-token", "", "Required Personal Access Token generated in Linode Console.")
	linodeRegionParamPtr  = config.String("linode-region", "", "Required linode region.")
	linodeLabelParamPtr   = config.String("linode-label", "", "Sets the Linode instance label.")
)

func main() {

	//
	config.AppName = "docker-volume-linode"
	config.Parse()

	//
	log.GetDefaultWriter().SetLevel(log.StrToLevel(*logLevelParamPtr))
	log.SetTrace(*logTraceParamPtr)

	// check required parameters (token, region and label)
	if len(*linodeTokenParamPtr) == 0 {
		log.Err("LINODE_TOKEN is required.")
		os.Exit(1)
	}

	if len(*linodeRegionParamPtr) == 0 {
		log.Err("LINODE_REGION is required.")
		os.Exit(1)
	}

	if len(*linodeLabelParamPtr) == 0 {
		log.Err("LINODE_LABEL is required.")
		os.Exit(1)
	}

	//
	log.Debug("LINODE_TOKEN: %s", *linodeTokenParamPtr)
	log.Debug("LINODE_REGION: %s", *linodeRegionParamPtr)
	log.Debug("LINODE_LABEL: %s", *linodeLabelParamPtr)

	// Linode API instance
	linodeAPI := linodego.NewClient(linodeTokenParamPtr, nil)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI, *linodeRegionParamPtr, linodeLabelParamPtr)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	serr := handler.ServeUnix(*socketAddressParamPtr, *socketGIDParamPtr)
	if serr != nil {
		log.Err("failed to bind to the Unix socket: %v", serr)
		os.Exit(1)
	}
}
