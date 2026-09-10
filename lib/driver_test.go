package dockerVolumeS3

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/docker/go-plugins-helpers/volume"
)

func testDriver() *S3fsDriver {
	return &S3fsDriver{
		mounts: make(map[string]int),
		conf:   map[string]string{"rootmount": "/mnt"},
	}
}

func TestPathUsesConfiguredRootMount(t *testing.T) {
	driver := testDriver()
	got, err := driver.Path(&volume.PathRequest{Name: "bucket"})
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got.Mountpoint != "/mnt/bucket" {
		t.Fatalf("Mountpoint = %q, want %q", got.Mountpoint, "/mnt/bucket")
	}
}

func TestCapabilitiesAreGlobal(t *testing.T) {
	got := testDriver().Capabilities()
	if got == nil || got.Capabilities.Scope != "global" {
		t.Fatalf("Capabilities() = %#v, want global scope", got)
	}
}

func TestMountReusesExistingMountAndIncrementsReferenceCount(t *testing.T) {
	driver := testDriver()
	driver.mounts["bucket"] = 1
	got, err := driver.Mount(&volume.MountRequest{Name: "bucket"})
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	if got.Mountpoint != "/mnt/bucket" {
		t.Fatalf("Mountpoint = %q, want %q", got.Mountpoint, "/mnt/bucket")
	}
	if got := driver.mounts["bucket"]; got != 2 {
		t.Fatalf("mount reference count = %d, want 2", got)
	}
}

func TestUnmountDecrementsSharedMountWithoutRunningUmount(t *testing.T) {
	driver := testDriver()
	driver.mounts["bucket"] = 2
	if err := driver.Unmount(&volume.UnmountRequest{Name: "bucket"}); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if got := driver.mounts["bucket"]; got != 1 {
		t.Fatalf("mount reference count = %d, want 1", got)
	}
}

func TestMountCreatesPathsAndTracksFirstMount(t *testing.T) {
	root := t.TempDir()
	var command string
	driver := testDriver()
	driver.conf["rootmount"] = root
	driver.conf["mountdir"] = "/data"
	driver.conf["s3fspath"] = "/usr/bin/s3fs"
	driver.runCommand = func(got string) error { command = got; return nil }
	response, err := driver.Mount(&volume.MountRequest{Name: "bucket"})
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}
	if response.Mountpoint != filepath.Join(root, "bucket", "data") {
		t.Fatalf("Mountpoint = %q, want %q", response.Mountpoint, filepath.Join(root, "bucket", "data"))
	}
	if command == "" || driver.mounts["bucket"] != 1 {
		t.Fatalf("command = %q, mount count = %d, want command and count 1", command, driver.mounts["bucket"])
	}
}

func TestMountDoesNotTrackFailedCommand(t *testing.T) {
	root := t.TempDir()
	driver := testDriver()
	driver.conf["rootmount"] = root
	driver.runCommand = func(string) error { return errors.New("mount failed") }
	if _, err := driver.Mount(&volume.MountRequest{Name: "bucket"}); err == nil {
		t.Fatal("Mount() error = nil, want command error")
	}
	if driver.mounts["bucket"] != 0 {
		t.Fatalf("mount count = %d, want 0 after failed mount", driver.mounts["bucket"])
	}
}

func TestUnmountRunsCommandForLastMount(t *testing.T) {
	root := t.TempDir()
	var command string
	driver := testDriver()
	driver.conf["rootmount"] = root
	driver.mounts["bucket"] = 1
	driver.runCommand = func(got string) error { command = got; return nil }
	if err := driver.Unmount(&volume.UnmountRequest{Name: "bucket"}); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if command != "umount "+filepath.Join(root, "bucket") || driver.mounts["bucket"] != 0 {
		t.Fatalf("command = %q, mount count = %d, want last unmount and count 0", command, driver.mounts["bucket"])
	}
}

func TestUnmountDoesNotDecrementUnmatchedMount(t *testing.T) {
	driver := testDriver()
	driver.runCommand = func(string) error { return nil }
	if err := driver.Unmount(&volume.UnmountRequest{Name: "never-mounted"}); err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if got := driver.mounts["never-mounted"]; got != 0 {
		t.Fatalf("mount count = %d, want 0", got)
	}
}

func TestUnmountKeepsReferenceCountWhenCommandFails(t *testing.T) {
	driver := testDriver()
	driver.mounts["bucket"] = 1
	driver.runCommand = func(string) error { return errors.New("umount failed") }
	if err := driver.Unmount(&volume.UnmountRequest{Name: "bucket"}); err == nil {
		t.Fatal("Unmount() error = nil, want command error")
	}
	if got := driver.mounts["bucket"]; got != 1 {
		t.Fatalf("mount count = %d, want 1 after failed unmount", got)
	}
}
