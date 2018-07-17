package main

import (
	"fmt"
	"os"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/config"
	"github.com/libgolang/docker-volume-linode/linode"
	"github.com/libgolang/log"
)

const socketAddress = "/run/docker/plugins/linode-driver.sock"

var (
	logLevelParamPtr = config.String("log.leve", "DEBUG", "Log Level. Defaults to WARN")
	logTraceParamPtr = config.Bool("log.trace", true, "Set Tracing to true")

	linodeTokenParamPtr = config.String("linode.token", "", "Required Personal Access Token generated in Linode Console.")
	regionParamPtr      = config.String("linode.region", "us-west", "Sets the cluster region")
	hostParamPtr        = config.String("linode.host", hostname(), "Sets the cluster region")
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
	log.Debug("LINODE_HOST: %s", *hostParamPtr)
	log.Debug("============================================================")

	// Linode API instance
	linodeAPI := linode.NewAPI(*linodeTokenParamPtr, *regionParamPtr, *hostParamPtr)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	fmt.Println(handler.ServeUnix(socketAddress, 0))
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
