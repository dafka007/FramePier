//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var shellExecuteEx = shell32.NewProc("ShellExecuteExW")

// SHELLEXECUTEINFOW uses native pointer alignment, including padding after Show.
type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon, Process                     uintptr
}

// Explorer is an intentional GUI action. Never send it through the background
// command constructor: HideWindow passes SW_HIDE to Explorer too.
func openWindowsFolder(path string) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- showWindowsFolder(path)
	}()
	return <-done
}

func showWindowsFolder(path string) error {
	// Validate again immediately before handing a directory to the Windows shell.
	validated, err := validateDownloadPath(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(validated)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("folder target is not a directory")
	}
	hr, _, _ := coInitializeEx.Call(0, coinitApartment|0x4) // COINIT_DISABLE_OLE1DDE
	if failedHRESULT(hr) {
		return fmt.Errorf("CoInitializeEx failed: 0x%08X", uint32(hr))
	}
	defer coUninitialize.Call()
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(validated)
	if err != nil {
		return err
	}
	request := shellExecuteInfo{
		Mask: 0x100 | 0x400,             // SEE_MASK_NOASYNC | SEE_MASK_FLAG_NO_UI
		Verb: verb, File: file, Show: 1, // SW_SHOWNORMAL; no argument string
	}
	request.Size = uint32(unsafe.Sizeof(request))
	ok, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&request)))
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	if ok == 0 {
		return fmt.Errorf("ShellExecuteExW failed (shell code %d): %w", request.Instance, callErr)
	}
	return nil
}

// Only helper-owned paths reach this function. The protocol accepts no path
// field for open_download_folder or open_logs_folder.
func (a *app) launchFixedFolder(path, action string) error {
	return a.launchFixedFolderWith(path, action, openWindowsFolder)
}

func (a *app) launchFixedFolderWith(path, action string, launch func(string) error) error {
	label := folderDiagnosticPath(path)
	info, statErr := os.Stat(path)
	a.logger.Printf("folder_open_requested action=%s path=%q exists=%t directory=%t", action, label, statErr == nil, statErr == nil && info.IsDir())
	validated, err := validateDownloadPath(path)
	if err == nil {
		err = os.MkdirAll(validated, 0700)
	}
	if err == nil {
		validated, err = validateDownloadPath(validated)
	}
	if err == nil {
		err = launch(validated)
	}
	if err != nil {
		diagnostic := strings.ReplaceAll(err.Error(), path, label)
		a.logger.Printf("folder_open_failed action=%s path=%q error=%q", action, label, diagnostic)
		return err
	}
	a.logger.Printf("folder_open_succeeded action=%s path=%q show=normal", action, label)
	return nil
}

func folderDiagnosticPath(path string) string {
	profile := os.Getenv("USERPROFILE")
	if profile != "" {
		relative, err := filepath.Rel(profile, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.Join("%USERPROFILE%", relative)
		}
	}
	return path
}
