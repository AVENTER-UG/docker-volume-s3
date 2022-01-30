package dockerVolumeS3

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/minio/minio-go/v6"
	log "github.com/sirupsen/logrus"
)

const (
	emptyVolume = `# s3vol configuration
# volumename;bucket;options
`
	configObject = "volumes"
	s3fspwdfile  = "/etc/passwd-s3fs"
)

//S3fsDriver is a volume driver over s3fs
type S3fsDriver struct {
	s3client   *minio.Client
	mounts     map[string]int
	mountsLock sync.Mutex
	conf       map[string]string // ceph config params
}

//VolConfig represents the configuration of a volume
type VolConfig struct {
	Name    string
	Bucket  string
	Options map[string]string
}

//NewDriver creates a new S3FS driver
func NewDriver() (*S3fsDriver, error) {

	driver := &S3fsDriver{
		mounts: make(map[string]int),
		conf:   make(map[string]string),
	}

	driver.configure()

	logLevel := "3"

	switch logLevel {
	case "3":
		log.SetLevel(log.DebugLevel)
		break
	case "2":
		log.SetLevel(log.InfoLevel)
		break
	case "1":
		log.SetLevel(log.WarnLevel)
		break
	default:
		log.SetLevel(log.ErrorLevel)
	}

	s3fspath := driver.conf["s3fspath"]
	if len(s3fspath) == 0 {
		path := os.Getenv("PATH")
		paths := strings.Split(path, ":")
		for _, p := range paths {
			log.WithField("command", "driver").Debugf("checking for s3fs in %s", p)
			info, err := os.Stat(fmt.Sprintf("%s/s3fs", p))
			if err != nil {
				log.WithField("command", "driver").Debugf("could not stat %s/s3fs: %s", p, err)
				continue
			}
			if info.IsDir() {
				log.WithField("command", "driver").Debugf("path %s/s3fs is a directory", p)
				continue
			}
			if !strings.Contains(info.Mode().String(), "x") {
				log.WithField("command", "driver").Debugf("file %s/s3fs is not executable (%s)", p, info.Mode().String())
				continue
			}
			log.WithField("command", "driver").Debugf("found s3fs path: %s/s3fs", p)
			s3fspath = fmt.Sprintf("%s/s3fs", p)
			break
		}
	}
	if len(s3fspath) == 0 {
		log.WithField("command", "driver").Errorf("could not get s3fs path: provide s3fs path or install it")
		return nil, fmt.Errorf("could not get s3fs path: provide s3fs path or install it")
	}
	driver.conf["s3fspath"] = s3fspath
	u, err := url.Parse(driver.conf["endpoint"])
	if err != nil {
		log.WithField("command", "driver").Errorf("could not parse endpoint: %s", err)
		return nil, fmt.Errorf("could not parse enpoint: %s", err)
	}
	endpoint := u.Host
	if u.Scheme != "https" && u.Scheme != "http" {
		log.WithField("command", "driver").Errorf("s3 scheme not http(s)")
		return nil, fmt.Errorf("s3 scheme not http(s)")
	}
	usessl := true
	if u.Scheme == "http" {
		usessl = false
	}
	accesskey := driver.conf["accesskey"]
	secretkey := driver.conf["secretkey"]
	region := driver.conf["region"]
	replaceunderscores := driver.conf["replaceunderscores"]
	configbucketname := driver.conf["configbucketname"]
	mount := driver.conf["mount"]
	mount = strings.TrimRight(mount, "/")
	defaults, err := parseOptions(driver.conf["options"])
	if err != nil {
		log.WithField("command", "driver").Errorf("could not parse options: %s", err)
		return nil, fmt.Errorf("could not parse options: %s", err)
	}
	// save s3fs password
	err = ioutil.WriteFile(s3fspwdfile, []byte(fmt.Sprintf("%s:%s", accesskey, secretkey)), 0660)
	if err != nil {
		log.WithField("command", "driver").Errorf("could not write s3fs password file: %s", err)
		return nil, fmt.Errorf("could not write s3fs password file: %s", err)
	}
	// add connection info to default options
	defaults["url"] = u.String()
	defaults["endpoint"] = region
	// default use path request style for minio
	defaults["use_path_request_style"] = "true"
	log.WithField("command", "driver").Infof("endpoint: %s", endpoint)
	log.WithField("command", "driver").Infof("use ssl: %v", usessl)
	log.WithField("command", "driver").Infof("access key: %s", accesskey)
	log.WithField("command", "driver").Infof("region: %s", region)
	log.WithField("command", "driver").Infof("replace underscores: %v", replaceunderscores)
	log.WithField("command", "driver").Infof("mount: %s", mount)
	log.WithField("command", "driver").Infof("config bucket: %s", configbucketname)
	log.WithField("command", "driver").Infof("default options: %s", defaults)
	// get a s3 client
	clt, err := minio.NewWithRegion(endpoint, accesskey, secretkey, usessl, region)
	if err != nil {
		log.WithField("command", "driver").Errorf("cannot get s3 client: %s", err)
		return nil, fmt.Errorf("cannot get s3 client: %s", err)
	}
	driver.s3client = clt
	err = driver.createBucket(configbucketname)
	if err != nil {
		log.WithField("command", "driver").Errorf("could check bucket '%s': %s", configbucketname, err)
		return nil, fmt.Errorf("could not check bucket '%s': %s", configbucketname, err)
	}
	// check config object existance
	_, err = clt.StatObject(configbucketname, configObject, minio.StatObjectOptions{})
	if err != nil {
		// create an empty config object
		reader := strings.NewReader(emptyVolume)
		err := driver.Lock(configbucketname, configObject)
		if err != nil {
			log.WithField("command", "driver").Errorf("could not lock config in %s: %s", configbucketname, err)
			return nil, fmt.Errorf("could not lock config in %s: %s", configbucketname, err)
		}
		_, err = clt.PutObject(configbucketname, configObject, reader, reader.Size(), minio.PutObjectOptions{})
		if err != nil {
			log.WithField("command", "driver").Errorf("could not create config in %s: %s", configbucketname, err)
			return nil, fmt.Errorf("could not create config in %s: %s", configbucketname, err)
		}
		err = driver.UnLock(configbucketname, configObject)
		if err != nil {
			log.WithField("command", "driver").Errorf("could not unlock config in %s: %s", configbucketname, err)
			return nil, fmt.Errorf("could not unlock config in %s: %s", configbucketname, err)
		}
	}
	// return the driver
	return driver, nil
}

//Create creates a volume
func (d *S3fsDriver) Create(req *volume.CreateRequest) error {
	log.WithField("command", "driver").WithField("method", "create").Debugf("request: %+v", req)
	// check bucket name
	bucket := req.Name
	if strings.Contains(bucket, "_") && d.conf["replaceunderscores"] == "true" {
		bucket = strings.ReplaceAll(bucket, "_", "-")
	}
	// check that the bucket exists
	err := d.createBucket(bucket)
	if err != nil {
		log.WithField("command", "driver").WithField("method", "create").Errorf("could check bucket '%s': %s", bucket, err)
		return fmt.Errorf("could check bucket '%s': %s", bucket, err)
	}
	return nil
}

//List lists volumes
func (d *S3fsDriver) List() (*volume.ListResponse, error) {
	log.WithField("command", "driver").WithField("method", "list").Debugf("list")
	// get bucket infos
	bucketInfos, err := d.s3client.ListBuckets()
	if err != nil {
		log.WithField("command", "driver").Errorf("could not get bucket infos: %s", err)
		return nil, fmt.Errorf("could not get bucket infos: %s", err)
	}
	resp := make([]*volume.Volume, len(bucketInfos))
	for i, v := range bucketInfos {
		// search for the bucket creation date
		creation := ""
		resp[i] = &volume.Volume{
			Name:       v.Name,
			Mountpoint: fmt.Sprintf("%s/%s", d.conf["rootmount"], v.Name),
			CreatedAt:  creation,
		}
		log.Info(v.Name)
	}
	return &volume.ListResponse{Volumes: resp}, nil
}

//Get gets a volume
func (d *S3fsDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	log.WithField("command", "driver").WithField("method", "get").Debugf("request: %+v", req)
	// get bucket infos
	bucketInfos, err := d.s3client.ListBuckets()
	if err != nil {
		log.WithField("command", "driver").WithField("method", "get").Errorf("could not get bucket infos: %s", err)
		return nil, fmt.Errorf("could not get bucket infos: %s", err)
	}
	// get creation date
	creation := ""
	for _, b := range bucketInfos {
		if req.Name == b.Name {
			creation = b.CreationDate.UTC().Format(time.RFC3339)
			break
		}
	}
	return &volume.GetResponse{
		Volume: &volume.Volume{
			Name:       req.Name,
			Mountpoint: fmt.Sprintf("%s/%s", d.conf["rootmount"], req.Name),
			CreatedAt:  creation,
		},
	}, nil
}

//Remove removes a volume
func (d *S3fsDriver) Remove(req *volume.RemoveRequest) error {
	log.WithField("command", "driver").WithField("method", "remove").Debugf("request: %+v", req)
	// check bucket
	buckets, err := d.s3client.ListBuckets()
	if err != nil {
		log.WithField("command", "driver").WithField("method", "remove").Errorf("could not list buckets: %s", err)
		return fmt.Errorf("could not list buckets: %s", err)
	}
	for _, bucket := range buckets {
		if bucket.Name == req.Name {
			log.WithField("command", "driver").WithField("method", "remove").Infof("removing bucket: %s", req.Name)
			// empty bucket
			// channel of objects to remove
			objectsCh := make(chan string)
			// Send object names that are needed to be removed to objectsCh
			go func() {
				defer close(objectsCh)
				// List all objects from a bucket
				for object := range d.s3client.ListObjects(req.Name, "", true, nil) {
					if object.Err != nil {
						log.WithField("command", "driver").WithField("method", "remove").Errorf("removing object from bucket '%s': %s", req.Name, object.Err)
						break
					}
					objectsCh <- object.Key
				}
			}()
			// remove the obtained objects from channel
			for rErr := range d.s3client.RemoveObjects(req.Name, objectsCh) {
				log.WithField("command", "driver").WithField("method", "remove").Errorf("error emptying bucket '%s': %s", req.Name, rErr)
				// don't exist: try to remove the bucket anyway
				break
			}
			// remove bucket
			err = d.s3client.RemoveBucket(req.Name)
			if err != nil {
				log.WithField("command", "driver").WithField("method", "remove").Errorf("could not remove bucket: %s", err)
				return fmt.Errorf("could not remove bucket: %s", err)
			}
			break
		}
	}
	return nil
}

//Path provides the path
func (d *S3fsDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	log.WithField("command", "driver").WithField("method", "path").Debugf("request: %+v", req)
	return &volume.PathResponse{Mountpoint: fmt.Sprintf("%s/%s", d.conf["rootmount"], req.Name)}, nil
}

//Mount mounts a volume
func (d *S3fsDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.WithField("command", "driver").WithField("method", "mount").Debugf("request: %+v", req)

	// generate mount path
	path := fmt.Sprintf("%s/%s", d.conf["rootmount"], req.Name)
	d.mountsLock.Lock()
	defer d.mountsLock.Unlock()
	if _, ok := d.mounts[req.Name]; ok {
		d.mounts[req.Name] = 0
	}
	if d.mounts[req.Name] > 0 {
		d.mounts[req.Name]++
		log.WithField("command", "driver").WithField("method", "mount").Infof("volume %s is used by %d containers", req.Name, d.mounts[req.Name])
		return &volume.MountResponse{Mountpoint: path}, nil
	}

	options := d.conf["options"]
	// create path if not exists
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		log.WithField("command", "driver").WithField("method", "mount").Errorf("could not get mount path %s: %s", path, err)
		return nil, fmt.Errorf("could not get mount path %s: %s", path, err)
	}
	if os.IsNotExist(err) {
		// create path
		err := os.Mkdir(path, 0770)
		if err != nil {
			log.WithField("command", "driver").WithField("method", "mount").Errorf("could not create mount path %s: %s", path, err)
			return nil, fmt.Errorf("could not create mount path %s: %s", path, err)
		}
	} else {
		if !info.IsDir() {
			log.WithField("command", "driver").WithField("method", "mount").Errorf("mount path %s is not a directory: %s", path, err)
			return nil, fmt.Errorf("mount path %s is not a directory: %s", path, err)
		}
	}
	// generate command
	cmd := fmt.Sprintf("%s %s %s -o %s", d.conf["s3fspath"], req.Name, path, options)
	log.WithField("command", "driver").WithField("method", "mount").Infof("cmd: %s", cmd)
	err = exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			if len(e.Stderr) > 0 {
				message := strings.ReplaceAll(string(e.Stderr), "\n", "\\n")
				log.WithField("command", "driver").WithField("method", "mount").Errorf("error executing the mount command: '%s'", message)
				return nil, fmt.Errorf("error executing the mount command: '%s'", message)
			}
			log.WithField("command", "driver").WithField("method", "mount").Errorf("error executing the mount command: %s", err)
			return nil, fmt.Errorf("error executing the mount command: %s", err)
		default:
			log.WithField("command", "driver").WithField("method", "mount").Errorf("error executing the mount command: %s", err)
			return nil, fmt.Errorf("error executing the mount command: %s", err)
		}
	}
	d.mounts[req.Name]++
	log.WithField("command", "driver").WithField("method", "mount").Infof("volume %s is used by %d containers", req.Name, d.mounts[req.Name])
	return &volume.MountResponse{Mountpoint: path}, nil
}

//Unmount unmounts a volume
func (d *S3fsDriver) Unmount(req *volume.UnmountRequest) error {
	log.WithField("command", "driver").WithField("method", "unmount").Debugf("request: %+v", req)
	// aquire mount lock
	d.mountsLock.Lock()
	defer d.mountsLock.Unlock()
	// check if other container still have this mounted
	if d.mounts[req.Name] > 1 {
		d.mounts[req.Name]--
		log.WithField("command", "driver").WithField("method", "unmount").Infof("volume %s is used by %d containers", req.Name, d.mounts[req.Name])
		return nil
	}
	// generate mount path
	path := fmt.Sprintf("%s/%s", d.conf["rootmount"], req.Name)
	// unmount volume
	// generate command
	cmd := fmt.Sprintf("umount %s", path)
	log.WithField("command", "driver").WithField("method", "unmount").Infof("cmd: %s", cmd)
	err := exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			if len(e.Stderr) > 0 {
				message := strings.ReplaceAll(string(e.Stderr), "\n", "\\n")
				log.WithField("command", "driver").WithField("method", "umount").Errorf("error executing the umount command: '%s'", message)
				return fmt.Errorf("error executing the umount command: '%s'", message)
			}
			log.WithField("command", "driver").WithField("method", "umount").Errorf("error executing the umount command: %s", err)
			return fmt.Errorf("error executing the umount command: %s", err)
		default:
			log.WithField("command", "driver").WithField("method", "umount").Errorf("error executing the umount command: %s", err)
			return fmt.Errorf("error executing the umount command: %s", err)
		}
	}
	d.mounts[req.Name]--
	log.WithField("command", "driver").WithField("method", "unmount").Infof("volume %s is used by %d containers", req.Name, d.mounts[req.Name])
	return nil
}

//Capabilities returns capabilities
func (d *S3fsDriver) Capabilities() *volume.CapabilitiesResponse {
	log.WithField("command", "driver").WithField("method", "capabilities").Debugf("scope: global")
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}
