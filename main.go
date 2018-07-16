package main

import (
	"os"

	"github.com/chiefy/linodego"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/config"
	"github.com/libgolang/log"
)

const (
	DefaultLinodeRegion  = "us-west"
	DefaultSocketAddress = "/run/docker/plugins/linode-driver.sock"
	DefaultSocketGID     = 0
	DefaultMountRoot     = "/mnt/"
)

var (
	logLevelParamPtr      = config.String("log-level", "DEBUG", "Log Level. Defaults to WARN")
	logTraceParamPtr      = config.Bool("log-trace", true, "Set Tracing to true")
	socketAddressParamPtr = config.String("socket-file", DefaultSocketAddress, "Sets the socket file/address")
	socketGIDParamPtr     = config.Int("socket-gid", DefaultSocketGID, "Sets the socket group id")
	mountRootParamPtr     = config.String("mount-root", DefaultMountRoot, "Sets the root directory for volume mounts")
	linodeTokenParamPtr   = config.String("linode-token", "", "Required Personal Access Token generated in Linode Console.")
	regionParamPtr        = config.String("linode-region", DefaultLinodeRegion, "Sets the cluster region")
	linodeLabelParamPtr   = config.String("linode-label", linodeLabel(), "Sets the Linode instance label")
)

func main() {
	//
	config.AppName = "docker-volume-linode"
	config.Parse()

	//
	log.GetDefaultWriter().SetLevel(log.StrToLevel(*logLevelParamPtr))
	log.SetTrace(*logTraceParamPtr)

	//
	log.Debug("============================================================")
	log.Debug("LINODE_TOKEN: %s", *linodeTokenParamPtr)
	log.Debug("LINODE_REGION: %s", *regionParamPtr)
	log.Debug("LINODE_LABEL: %s", *linodeLabelParamPtr)
	log.Debug("============================================================")

	if len(*linodeTokenParamPtr) == 0 {
		log.Err("LINODE_TOKEN is required.")
		os.Exit(1)
	}

	// Linode API instance
	linodeAPI := linodego.NewClient(linodeTokenParamPtr, nil)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI, *regionParamPtr, linodeLabelParamPtr)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	serr := handler.ServeUnix(*socketAddressParamPtr, *socketGIDParamPtr)
	if serr != nil {
		log.Err("failed to bind to the Unix socket: %v", serr)
		os.Exit(1)
	}

	for {

	}
}

// linodeLabel determines the instance label of the Linode where this volume driver is running
func linodeLabel() string {
	h, _ := os.Hostname()
	return h
}
