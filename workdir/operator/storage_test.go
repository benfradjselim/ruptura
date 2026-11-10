package main

import (
	"strings"
	"testing"
)

func TestDataVolume(t *testing.T) {
	tests := []struct {
		name     string
		spec     RupturaInstanceSpec
		wantPVC  bool
		wantSize string
	}{
		{"volume claim by default", RupturaInstanceSpec{}, true, ""},
		{"emptyDir with the default size", RupturaInstanceSpec{EphemeralStorage: true}, false, defaultStorageSize},
		{"emptyDir bounded by storageSize", RupturaInstanceSpec{EphemeralStorage: true, StorageSize: "2Gi"}, false, "2Gi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := dataVolume("ruptura", tt.spec)
			if v.Name != "data" {
				t.Fatalf("name = %q", v.Name)
			}
			if tt.wantPVC {
				if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "ruptura-data" || v.EmptyDir != nil {
					t.Fatalf("volume = %+v, want the ruptura-data claim only", v)
				}
				return
			}
			if v.EmptyDir == nil || v.PersistentVolumeClaim != nil || v.EmptyDir.SizeLimit != tt.wantSize {
				t.Fatalf("volume = %+v, want an emptyDir of %s only", v, tt.wantSize)
			}
		})
	}
}

func TestStatusMessage(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"kubernetes status", `{"kind":"Status","message":"spec.template.spec.volumes[0]: may not specify more than 1 volume type"}`, "may not specify more than 1 volume type"},
		{"plain text", "  upstream connect error \n", "upstream connect error"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusMessage(strings.NewReader(tt.body)); !strings.Contains(got, tt.want) || (tt.want == "" && got != "") {
				t.Fatalf("statusMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
