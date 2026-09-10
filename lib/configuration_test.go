package dockerVolumeS3

import "testing"

func TestConfigureLoadsDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("S3_CONF_REGION", "eu-west-1")
	t.Setenv("S3_CONF_CUSTOM_FLAG", "enabled")
	driver := &S3fsDriver{conf: make(map[string]string)}

	driver.configure()

	wantDefaults := map[string]string{
		"endpoint":           "http://",
		"region":             "eu-west-1",
		"rootmount":          "/mnt",
		"replaceunderscores": "true",
		"usessl":             "true",
		"mountdir":           "/data",
	}
	for key, want := range wantDefaults {
		if got := driver.conf[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if got := driver.conf["custom_flag"]; got != "enabled" {
		t.Fatalf("custom_flag = %q, want environment value", got)
	}
}

func TestLoadEnvironmentS3ConfigVarsIgnoresOtherVariables(t *testing.T) {
	t.Setenv("NOT_S3_CONF_VALUE", "ignored")
	t.Setenv("S3_CONF_MIXED_CASE", "value")
	driver := &S3fsDriver{conf: make(map[string]string)}

	driver.loadEnvironmentS3ConfigVars()

	if _, ok := driver.conf["not_s3_conf_value"]; ok {
		t.Fatal("non-S3_CONF variable was loaded")
	}
	if got := driver.conf["mixed_case"]; got != "value" {
		t.Fatalf("mixed_case = %q, want %q", got, "value")
	}
}

func TestLoadEnvironmentPreservesValuesContainingEquals(t *testing.T) {
	t.Setenv("S3_CONF_TOKEN", "a=b=c")
	driver := &S3fsDriver{conf: make(map[string]string)}

	driver.loadEnvironmentS3ConfigVars()

	if got := driver.conf["token"]; got != "a=b=c" {
		t.Fatalf("token = %q, want %q", got, "a=b=c")
	}
}
