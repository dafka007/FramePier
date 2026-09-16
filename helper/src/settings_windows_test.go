package main

import (
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
