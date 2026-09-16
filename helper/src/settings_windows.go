package main

import (
	"os"
	"path/filepath"
)

// Keep the previous settings intact if a write is interrupted or fails.
// Each helper connection gets its own staging file in the same directory.
func writeSettingsAtomically(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(file.Name(), path)
}
