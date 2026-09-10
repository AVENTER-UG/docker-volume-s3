package dockerVolumeS3

import (
	"strings"
	"testing"
)

func TestNewDriverFailsWhenS3FSIsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("S3_CONF_S3FSPATH", "")

	_, err := NewDriver()
	if err == nil || !strings.Contains(err.Error(), "could not get s3fs path") {
		t.Fatalf("NewDriver() error = %v, want missing s3fs error", err)
	}
}

func TestNewDriverRejectsUnsupportedEndpointScheme(t *testing.T) {
	t.Setenv("S3_CONF_S3FSPATH", "/usr/bin/s3fs")
	t.Setenv("S3_CONF_ENDPOINT", "ftp://storage.example.test")

	_, err := NewDriver()
	if err == nil || !strings.Contains(err.Error(), "s3 scheme not http(s)") {
		t.Fatalf("NewDriver() error = %v, want endpoint scheme error", err)
	}
}
