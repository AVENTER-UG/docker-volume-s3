package dockerVolumeRbd

import (
	"os"
	"strings"
)

// Read
func (d *S3fsDriver) configure() {

	// set default confs:

	d.conf["endpoint"] = "http://"
	d.conf["region"] = "us-east-1"
	d.conf["mount"] = "/mnt"
	d.conf["replaceunderscores"] = true
	d.conf["configbucket"] = "s3volconfig"

	d.loadEnvironmentS3ConfigVars

}

// Get only the env vars starting by S3_CONF_*
// i.e. S3_CONF_GLOBAL_MON_HOST is saved in d.conf[global_mon_host]
//
func (d *S3fsDriver) loadEnvironmentS3ConfigVars() {
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)

		if strings.HasPrefix(pair[0], "S3_CONF_") {
			configPair := strings.Split(pair[0], "S3_CONF_")
			d.conf[strings.ToLower(configPair[1])] = pair[1]
		}
	}

}
