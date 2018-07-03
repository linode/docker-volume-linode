package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/libgolang/docker-volume-linode/linode"
	"github.com/libgolang/log"
)

var (
	token            string
	logLevelParamPtr = param("log.leve", "DEBUG", "Log Level")
	logTraceParamPtr = flag.Bool("log.trace", false, "Set Tracing to true")

	linodeTokenParamPtr = param("linode.token", "", "Required Personal Access Token generated in Linode Console.")
	hostParamPtr        = param("host", hostname(), "Sets the cluster region")
	regionParamPtr      = param("region", "us-west", "Sets the cluster region")
)

func main() {
	flag.Parse()

	//
	//
	log.SetTrace(true)
	w := &log.WriterStdout{}
	w.SetLevel(log.DEBUG)
	log.SetWriters([]log.Writer{w})

	//
	// Get token from `-linode.token` param or from LINODE_TOKEN environment variable
	//
	if *linodeTokenParamPtr != "" {
		token = *linodeTokenParamPtr
	} else {
		token = os.Getenv("LINODE_TOKEN")
	}
	os.Setenv("LOG_CONFIG", "")
	log.Debug("Using linode token: %s", token)

	// Linode API instance
	linodeAPI := linode.NewAPI(token, *regionParamPtr, *hostParamPtr)

	// Driver instance
	driver := newLinodeVolumeDriver(linodeAPI)

	// Attach Driver to docker
	handler := volume.NewHandler(driver)
	fmt.Println(handler.ServeUnix("linode-volume-driver", 0))
}

func param(key, def, help string) *string {
	envKey := strings.Replace(strings.ToUpper(key), ".", "_", -1)
	return flag.String(key, getEnv(envKey, def), help)
}

func getEnv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
