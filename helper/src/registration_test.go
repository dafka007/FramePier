package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeHostManifestRoundTripWithSpaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "FramePier Test Install")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "FramePierHelper.exe"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeNativeHostManifest(root); err != nil {
		t.Fatal(err)
	}
	if err := validateNativeHostManifest(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifestPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var manifest nativeHostManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != nativeHostName || manifest.Path != filepath.Join(root, "FramePierHelper.exe") || len(manifest.AllowedOrigins) != 1 || manifest.AllowedOrigins[0] != extensionOrigin {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestManifestRepairCorrectsWrongPathAndOrigin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "FramePierHelper.exe"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath(root)), 0700); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"name":"com.viddock.helper","description":"bad","path":"C:\\\\Wrong.exe","type":"stdio","allowed_origins":["chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/"]}`)
	if err := os.WriteFile(manifestPath(root), bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateNativeHostManifest(root); err == nil {
		t.Fatal("corrupt manifest unexpectedly validated")
	}
	if err := writeNativeHostManifest(root); err != nil {
		t.Fatal(err)
	}
	if err := validateNativeHostManifest(root); err != nil {
		t.Fatal(err)
	}
}
