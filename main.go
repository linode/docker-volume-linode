package main

import (
	"os"
	"strconv"
	"strings"

	"flag"

	"github.com/docker/docker/client"
	"github.com/docker/go-plugins-helpers/volume"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultSocketAddress docket file to be created for communication with docker
	DefaultSocketAddress = "/run/docker/plugins/linode.sock"
	// DefaultSocketGID Group ownership of DefaultSocketAddress
	DefaultSocketGID = 0
)

// VERSION set by --ldflags "-X main.VERSION=$VERSION"
var VERSION string

var (
	// MountRoot mount point for all volumes
	MountRoot             = "/mnt"
	socketGIDParamPtr     = cfgInt("socket-gid", DefaultSocketGID, "Sets the socket group id.")
	socketAddressParamPtr = cfgString("socket-file", DefaultSocketAddress, "Sets the socket file/address.")
	mountRootParamPtr     = cfgString("mount-root", MountRoot, "Sets the root directory for volume mounts.")
	linodeTokenParamPtr   = cfgString("linode-token", "", "Required Personal Access Token generated in Linode Console.")
	linodeLabelParamPtr   = cfgString("linode-label", "", "Sets the Linode Instance Label (defaults to the OS HOSTNAME)")
	logLevelPtr           = cfgString("log-level", "info", "Sets log level: debug,info,warn,error")
)

func main() {
	//
	flag.Parse()
	//
	log.SetOutput(os.Stdout)
	level, err := log.ParseLevel(*logLevelPtr)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	log.Infof("docker-volume-linode/%s", VERSION)

	// check required parameters (token and label)
	if *linodeTokenParamPtr == "" {
		log.Fatal("linode-token is required.")
	}

	MountRoot = *mountRootParamPtr

	log.Debugf("linode-token: %s", *linodeTokenParamPtr)
	log.Debugf("linode-label: %s", *linodeLabelParamPtr)

	//
	docker, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	defer func() { _ = docker.Close() }()

	// Driver instance
	driver := newLinodeVolumeDriver(*linodeLabelParamPtr, *linodeTokenParamPtr, docker)

	// Attach Driver to docker
	handler := volume.NewHandler(&driver)
	log.Debug("connecting to socket ", *socketAddressParamPtr)
	serr := handler.ServeUnix(*socketAddressParamPtr, *socketGIDParamPtr)
	if serr != nil {
		log.Errorf("failed to bind to the Unix socket: %v", serr)
		os.Exit(1)
	}
}

func cfgString(name string, def string, desc string) *string {
	newDef := def
	if val, found := getEnv(name); found {
		newDef = val
	}
	return flag.String(name, newDef, desc)
}

func cfgInt(name string, def int, desc string) *int {
	newDef := def
	if val, found := getEnv(name); found {
		if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
			newDef = int(intVal)
		}
	}
	return flag.Int(name, newDef, desc)
}

func getEnv(name string) (string, bool) {
	if val, found := os.LookupEnv(name); found {
		return val, true
	}

	name = strings.ToUpper(name)
	name = strings.Replace(name, "-", "_", -1)

	if val, found := os.LookupEnv(name); found {
		return val, true
	}

	return "", false
}
