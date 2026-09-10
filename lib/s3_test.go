package dockerVolumeS3

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/minio/minio-go/v6"
)

type fakeS3Client struct {
	bucketExists    bool
	bucketExistsErr error
	makeBucketErr   error
	buckets         []minio.BucketInfo
	listBucketsErr  error
	makeBucketCalls []string
	removeBucketErr error
	removeCalls     []string
}

func (f *fakeS3Client) BucketExists(string) (bool, error) { return f.bucketExists, f.bucketExistsErr }
func (f *fakeS3Client) MakeBucket(bucket, region string) error {
	f.makeBucketCalls = append(f.makeBucketCalls, bucket+"@"+region)
	return f.makeBucketErr
}
func (f *fakeS3Client) ListBuckets() ([]minio.BucketInfo, error) { return f.buckets, f.listBucketsErr }
func (f *fakeS3Client) ListObjects(string, string, bool, <-chan struct{}) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo)
	close(ch)
	return ch
}
func (f *fakeS3Client) RemoveObjects(string, <-chan string) <-chan minio.RemoveObjectError {
	ch := make(chan minio.RemoveObjectError)
	close(ch)
	return ch
}
func (f *fakeS3Client) RemoveBucket(bucket string) error {
	f.removeCalls = append(f.removeCalls, bucket)
	return f.removeBucketErr
}
func (f *fakeS3Client) StatObject(string, string, minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return minio.ObjectInfo{}, errors.New("not implemented")
}
func (f *fakeS3Client) GetObject(string, string, minio.GetObjectOptions) (*minio.Object, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeS3Client) PutObject(string, string, io.Reader, int64, minio.PutObjectOptions) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *fakeS3Client) RemoveObject(string, string) error { return nil }

func TestCreateBucketDoesNotRecreateExistingBucket(t *testing.T) {
	client := &fakeS3Client{bucketExists: true}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"region": "eu-west-1"}}

	if err := driver.createBucket("existing"); err != nil {
		t.Fatalf("createBucket() error = %v", err)
	}
	if len(client.makeBucketCalls) != 0 {
		t.Fatalf("MakeBucket calls = %d, want 0", len(client.makeBucketCalls))
	}
}

func TestCreateBucketCreatesMissingBucket(t *testing.T) {
	client := &fakeS3Client{}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"region": "eu-west-1"}}

	if err := driver.createBucket("new-bucket"); err != nil {
		t.Fatalf("createBucket() error = %v", err)
	}
	if len(client.makeBucketCalls) != 1 || client.makeBucketCalls[0] != "new-bucket@eu-west-1" {
		t.Fatalf("MakeBucket calls = %#v, want one call with bucket and region", client.makeBucketCalls)
	}
}

func TestCreateReplacesUnderscoresWhenConfigured(t *testing.T) {
	client := &fakeS3Client{}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"region": "us-east-1", "replaceunderscores": "true"}}

	if err := driver.Create(&volume.CreateRequest{Name: "team_bucket"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if client.makeBucketCalls[0] != "team-bucket@us-east-1" {
		t.Fatalf("MakeBucket bucket = %q, want %q", client.makeBucketCalls[0], "team-bucket@us-east-1")
	}
}

func TestCreatePropagatesBucketErrors(t *testing.T) {
	wantErr := errors.New("bucket service unavailable")
	client := &fakeS3Client{bucketExistsErr: wantErr}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"replaceunderscores": "false"}}

	if err := driver.Create(&volume.CreateRequest{Name: "bucket"}); err == nil {
		t.Fatal("Create() error = nil, want bucket service error")
	}
}

func TestListBuildsVolumeResponses(t *testing.T) {
	client := &fakeS3Client{buckets: []minio.BucketInfo{{Name: "one"}, {Name: "two"}}}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"rootmount": "/mnt"}}

	response, err := driver.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(response.Volumes) != 2 || response.Volumes[1].Mountpoint != "/mnt/two" {
		t.Fatalf("List() volumes = %#v, want two volumes with configured mountpoints", response.Volumes)
	}
}

func TestListPropagatesBucketListingError(t *testing.T) {
	client := &fakeS3Client{listBucketsErr: errors.New("list failed")}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"rootmount": "/mnt"}}

	if _, err := driver.List(); err == nil {
		t.Fatal("List() error = nil, want listing error")
	}
}

func TestGetIncludesCreationDateForKnownBucket(t *testing.T) {
	created := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	client := &fakeS3Client{buckets: []minio.BucketInfo{{Name: "known", CreationDate: created}}}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{"rootmount": "/mnt"}}

	response, err := driver.Get(&volume.GetRequest{Name: "known"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if response.Volume.CreatedAt != created.Format(time.RFC3339) {
		t.Fatalf("CreatedAt = %q, want %q", response.Volume.CreatedAt, created.Format(time.RFC3339))
	}
}

func TestRemoveRemovesExistingBucket(t *testing.T) {
	client := &fakeS3Client{buckets: []minio.BucketInfo{{Name: "bucket"}}}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{}}

	if err := driver.Remove(&volume.RemoveRequest{Name: "bucket"}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if len(client.removeCalls) != 1 || client.removeCalls[0] != "bucket" {
		t.Fatalf("RemoveBucket calls = %#v, want bucket", client.removeCalls)
	}
}

func TestRemovePropagatesBucketRemovalError(t *testing.T) {
	client := &fakeS3Client{buckets: []minio.BucketInfo{{Name: "bucket"}}, removeBucketErr: errors.New("remove failed")}
	driver := &S3fsDriver{s3client: client, conf: map[string]string{}}

	if err := driver.Remove(&volume.RemoveRequest{Name: "bucket"}); err == nil {
		t.Fatal("Remove() error = nil, want removal error")
	}
}
