//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

func hasReparsePoint(path string) bool {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return true
			}
			if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				return true
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}
