package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsReplacementAndStagingCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	for _, contents := range []string{`{"downloadPath":"first"}`, `{"downloadPath":"second"}`} {
		if err := writeSettingsAtomically(path, []byte(contents)); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != contents {
			t.Fatalf("settings mismatch: %q, %v", data, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging files remain: %v, %v", entries, err)
	}
}

func TestLoadSettingsFreshInstallUsesFramePierDefault(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	dataDir := t.TempDir()
	// Simulate an unrelated existing Downloads\VidDock folder that should be ignored
	if err := os.MkdirAll(filepath.Join(profile, "Downloads", "VidDock"), 0700); err != nil {
		t.Fatal(err)
	}
	app := &app{dataDir: dataDir}
	s := app.loadSettings()
	want := filepath.Join(userDownloads(), "FramePier")
	if s.DownloadPath != want {
		t.Fatalf("DownloadPath = %q, want %q", s.DownloadPath, want)
	}
}

func TestLoadSettingsUpgradedVidDockKeepsOldDefault(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	dataDir := t.TempDir()
	// Simulate the installer's migration marker
	if err := os.WriteFile(filepath.Join(dataDir, ".viddock-upgraded"), []byte("upgraded"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &app{dataDir: dataDir}
	s := app.loadSettings()
	want := filepath.Join(userDownloads(), "VidDock")
	if s.DownloadPath != want {
		t.Fatalf("DownloadPath = %q, want %q", s.DownloadPath, want)
	}
}

func TestLoadSettingsCustomPersistedPathUnchanged(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	dataDir := t.TempDir()
	// Simulate the installer's migration marker (upgrade detected)
	if err := os.WriteFile(filepath.Join(dataDir, ".viddock-upgraded"), []byte("upgraded"), 0600); err != nil {
		t.Fatal(err)
	}
	// Simulate a persisted custom download path
	custom := filepath.Join(t.TempDir(), "custom-downloads")
	settingsJSON := fmt.Sprintf(`{"downloadPath":%q,"autoOpen":false}`, custom)
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(settingsJSON), 0600); err != nil {
		t.Fatal(err)
	}
	app := &app{dataDir: dataDir}
	s := app.loadSettings()
	if s.DownloadPath != custom {
		t.Fatalf("DownloadPath = %q, want %q", s.DownloadPath, custom)
	}
}
