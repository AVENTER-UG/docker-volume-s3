package main

import (
	"os"

	dockerVolumeS3 "github.com/AVENTER-UG/docker-volume-s3/lib"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/s3.sock"

func main() {
	dockerVolumeS3Version := os.Getenv("PLUGIN_VERSION")

	logLevel := os.Getenv("LOG_LEVEL")

	switch logLevel {
	case "3":
		logrus.SetLevel(logrus.DebugLevel)
		break
	case "2":
		logrus.SetLevel(logrus.InfoLevel)
		break
	case "1":
		logrus.SetLevel(logrus.WarnLevel)
		break
	default:
		logrus.SetLevel(logrus.ErrorLevel)
	}

	volDriver, err := dockerVolumeS3.NewDriver(c)
	if err != nil {
		logrus.Fatal(err)
	}

	h := volume.NewHandler(volDriver)
	logrus.Infof("plugin(s3) version(%s) started with log level(%s) attending socket(%s)", dockerVolumeS3Version, logLevel, socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))
	logrus.Error(h.ServeUnix(c.String("socket"), 0))
}
