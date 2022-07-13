package main

import (
	"flag"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	log "github.com/sirupsen/logrus"
)

// VERSION set by --ldflags "-X main.VERSION=$VERSION"
var VERSION string

var (
	forceAttach = cfgBool("force-attach", false, "If true, volumes will be forcibly attached to the current Linode if already attached to another Linode.")
	mountRoot   = cfgString("mount-root", "/mnt", "The location to mount volumes to.")
	socketUser  = cfgString("socket-user", "root", "Sets the user to create the socket with.")
	logLevel    = cfgString("log-level", "info", "Sets log level: debug,info,warn,error")
	linodeToken = cfgString("linode-token", "", "Required Personal Access Token generated in Linode Console.")
	linodeLabel = cfgString("linode-label", "", "Sets the Linode Instance Label (defaults to the OS HOSTNAME)")
)

func main() {
	flag.Parse()

	log.SetOutput(os.Stdout)
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	log.Infof("docker-volume-linode/%s", VERSION)

	// check required parameters (token and label)
	if *linodeToken == "" {
		log.Fatal("linode-token is required.")
	}

	log.Debugf("linode-token: %s", *linodeToken)
	log.Debugf("linode-label: %s", *linodeLabel)

	driver := newLinodeVolumeDriver(*linodeLabel, *linodeToken, *mountRoot)
	handler := volume.NewHandler(&driver)
	log.Debug("connecting to socket ", *socketUser)
	u, _ := user.Lookup(*socketUser)
	gid, _ := strconv.Atoi(u.Gid)
	log.Println(handler.ServeUnix("linode", gid))
	//if serr != nil {
	//	log.Errorf("failed to bind to the Unix socket: %v", serr)
	//	os.Exit(1)
	//}
}

func cfgString(name string, def string, desc string) *string {
	newDef := def
	if val, found := getEnv(name); found {
		newDef = val
	}
	return flag.String(name, newDef, desc)
}

func cfgBool(name string, def bool, desc string) bool {
	val, found := getEnv(name)
	if !found {
		return false
	}

	valNormalized := strings.ToLower(val)
	return valNormalized == "true" || valNormalized == "1"
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
