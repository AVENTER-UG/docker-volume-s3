package dockerVolumeS3

import (
	"reflect"
	"testing"
)

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{name: "empty", input: "", want: map[string]string{}},
		{name: "flags and values", input: "allow_other,uid=1000,disabled=false", want: map[string]string{"allow_other": "true", "uid": "1000"}},
		{name: "value containing equals", input: "password=a=b", want: map[string]string{"password": "a=b"}},
		{name: "duplicate uses last value", input: "uid=1000,uid=2000", want: map[string]string{"uid": "2000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptions(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseOptions() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestOptionsToStringSortsAndOmitsFalse(t *testing.T) {
	got := optionsToString(map[string]string{
		"z": "last",
		"a": "true",
		"m": "false",
		"b": "",
	})
	if got != "a,b,z=last" {
		t.Fatalf("optionsToString() = %q, want %q", got, "a,b,z=last")
	}
}

func TestOptionsToStringHandlesNil(t *testing.T) {
	if got := optionsToString(nil); got != "" {
		t.Fatalf("optionsToString(nil) = %q, want empty string", got)
	}
}
