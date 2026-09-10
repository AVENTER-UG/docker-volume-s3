package dockerVolumeS3

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLockCreatesLockWhenAbsent(t *testing.T) {
	var putBucket, putObject, putContent string
	driver := testDriver()
	driver.statObject = func(string, string) error { return errors.New("not found") }
	driver.putObject = func(bucket, object string, reader io.Reader, _ int64) error {
		body, err := io.ReadAll(reader)
		putBucket, putObject, putContent = bucket, object, string(body)
		return err
	}

	if err := driver.Lock("bucket", "object"); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	hostname, _ := os.Hostname()
	if putBucket != "bucket" || putObject != "object.ext.lock" || putContent != hostname {
		t.Fatalf("put = (%q, %q, %q), want bucket, lock name and hostname", putBucket, putObject, putContent)
	}
}

func TestLockPropagatesPutError(t *testing.T) {
	driver := testDriver()
	driver.statObject = func(string, string) error { return errors.New("not found") }
	driver.putObject = func(string, string, io.Reader, int64) error { return errors.New("put failed") }

	if err := driver.Lock("bucket", "object"); err == nil {
		t.Fatal("Lock() error = nil, want put error")
	}
}

func TestUnlockWithoutLockIsSuccessful(t *testing.T) {
	driver := testDriver()
	driver.statObject = func(string, string) error { return errors.New("not found") }

	if err := driver.UnLock("bucket", "object"); err != nil {
		t.Fatalf("UnLock() error = %v, want nil", err)
	}
}

func TestUnlockRejectsLockOwnedByAnotherHost(t *testing.T) {
	driver := testDriver()
	driver.statObject = func(string, string) error { return nil }
	driver.readObject = func(string, string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("another-host")), nil
	}

	if err := driver.UnLock("bucket", "object"); err == nil {
		t.Fatal("UnLock() error = nil, want ownership error")
	}
}

func TestUnlockRemovesOwnLock(t *testing.T) {
	removed := false
	hostname, _ := os.Hostname()
	driver := testDriver()
	driver.statObject = func(string, string) error { return nil }
	driver.readObject = func(string, string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(hostname)), nil
	}
	driver.removeObject = func(bucket, object string) error {
		removed = bucket == "bucket" && object == "object.ext.lock"
		return nil
	}

	if err := driver.UnLock("bucket", "object"); err != nil {
		t.Fatalf("UnLock() error = %v", err)
	}
	if !removed {
		t.Fatal("own lock was not removed")
	}
}
