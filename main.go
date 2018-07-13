package main

import (
	"fmt"
	"os"

	"github.com/chiefy/linodego"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/config"
	"github.com/libgolang/log"
)

const socketAddress = "/run/docker/plugins/linode-driver.sock"

var (
	logLevelParamPtr = config.String("log.leve", "DEBUG", "Log Level. Defaults to WARN")
	logTraceParamPtr = config.Bool("log.trace", true, "Set Tracing to true")

	linodeTokenParamPtr = config.String("linode.token", "", "Required Personal Access Token generated in Linode Console.")
	regionParamPtr      = config.String("linode.region", "us-west", "Sets the cluster region")
	linodeLabelParamPtr = config.String("linode.linode-label", linodeLabel(), "Sets the Linode instance label")
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

	// Linode API instance
	linodeAPI := linodego.NewClient(linodeTokenParamPtr, nil)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI, *regionParamPtr, linodeLabelParamPtr)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	fmt.Println(handler.ServeUnix(socketAddress, 0))
}

// linodeLabel determines the instance label of the Linode where this volume driver is running
func linodeLabel() string {
	h, _ := os.Hostname()
	return h
}
