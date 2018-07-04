package main

import (
	"fmt"
	"os"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/config"
	"github.com/libgolang/docker-volume-linode/linode"
	"github.com/libgolang/log"
)

var (
	token               string
	logLevelParamPtr    = config.String("log.leve", "INFO", "Log Level. Defaults to WARN")
	logTraceParamPtr    = config.Bool("log.trace", false, "Set Tracing to true")
	linodeTokenParamPtr = config.String("linode.token", "", "Required Personal Access Token generated in Linode Console.")
	hostParamPtr        = config.String("linode.host", hostname(), "Sets the cluster region")
	regionParamPtr      = config.String("linode.region", "us-west", "Sets the cluster region")
)

func main() {
	config.AppName = "docker-volume-linode"
	config.Parse()

	//
	log.GetDefaultWriter().SetLevel(log.StrToLevel(*logLevelParamPtr))
	log.SetTrace(*logTraceParamPtr)

	// Linode API instance
	linodeAPI := linode.NewAPI(token, *regionParamPtr, *hostParamPtr)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	fmt.Println(handler.ServeUnix("linode-driver", 0))
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
