//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func availableDiskBytes(path string) (uint64, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available uint64
	ok, _, callErr := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(pointer)), uintptr(unsafe.Pointer(&available)), 0, 0)
	if ok == 0 {
		return 0, callErr
	}
	return available, nil
}

func ensureEstimatedDiskSpace(downloadPath, tempPath string, estimate int64) error {
	if estimate <= 0 {
		return nil
	}
	const reserve = uint64(128 * 1024 * 1024)
	estimated := uint64(estimate)
	if estimated > (^uint64(0)-reserve)/2 {
		return errors.New("The estimated download is too large.")
	}
	tempRequired := estimated*2 + reserve
	downloadRequired := estimated + reserve
	tempAvailable, tempErr := availableDiskBytes(tempPath)
	downloadAvailable, downloadErr := availableDiskBytes(downloadPath)
	if tempErr != nil || downloadErr != nil {
		return nil
	}
	sameVolume := strings.EqualFold(filepath.VolumeName(tempPath), filepath.VolumeName(downloadPath))
	if sameVolume {
		if tempAvailable < tempRequired {
			return errors.New("There may not be enough free disk space for this download and its temporary processing files.")
		}
		return nil
	}
	if tempAvailable < tempRequired || downloadAvailable < downloadRequired {
		return errors.New("There may not be enough free disk space for this download and its temporary processing files.")
	}
	return nil
}
