//go:build windows

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type comGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type comObject struct {
	VTable *[32]uintptr
}

var (
	clsidFileOpenDialog  = comGUID{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog    = comGUID{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidShellItem         = comGUID{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
	ole32                = syscall.NewLazyDLL("ole32.dll")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	coInitializeEx       = ole32.NewProc("CoInitializeEx")
	coUninitialize       = ole32.NewProc("CoUninitialize")
	coCreateInstance     = ole32.NewProc("CoCreateInstance")
	coTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	shCreateItem         = shell32.NewProc("SHCreateItemFromParsingName")
	user32               = syscall.NewLazyDLL("user32.dll")
	folderPickerKernel32 = syscall.NewLazyDLL("kernel32.dll")
	getForegroundWindow  = user32.NewProc("GetForegroundWindow")
	getAncestor          = user32.NewProc("GetAncestor")
	isWindow             = user32.NewProc("IsWindow")
	isWindowVisible      = user32.NewProc("IsWindowVisible")
	getWindowProcessID   = user32.NewProc("GetWindowThreadProcessId")
	getClassName         = user32.NewProc("GetClassNameW")
	setForegroundWindow  = user32.NewProc("SetForegroundWindow")
	bringWindowToTop     = user32.NewProc("BringWindowToTop")
	getCurrentProcessID  = folderPickerKernel32.NewProc("GetCurrentProcessId")
	processIDToSession   = folderPickerKernel32.NewProc("ProcessIdToSessionId")
)

const (
	clsctxInprocServer   = 0x1
	coinitApartment      = 0x2
	fosNoChangeDir       = 0x8
	fosPickFolders       = 0x20
	fosForceFileSystem   = 0x40
	fosPathMustExist     = 0x800
	sigdnFileSystemPath  = 0x80058000
	hresultCancelled     = 0x800704C7
	hresultChangedMode   = 0x80010106
	fileDialogSetOptions = 9
	fileDialogSetFolder  = 12
	fileDialogSetTitle   = 17
	fileDialogGetResult  = 20
	shellItemDisplayName = 5
	getAncestorRoot      = 2
)

func chooseWindowsFolder(currentPath string, owner uintptr) (string, bool, error) {
	type result struct {
		path      string
		cancelled bool
		err       error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		path, cancelled, err := showFolderDialog(currentPath, owner)
		done <- result{path: path, cancelled: cancelled, err: err}
	}()
	value := <-done
	return value.path, value.cancelled, value.err
}

func showFolderDialog(currentPath string, owner uintptr) (string, bool, error) {
	hr, _, _ := coInitializeEx.Call(0, coinitApartment)
	if failedHRESULT(hr) {
		if uint32(hr) == hresultChangedMode {
			return "", false, errors.New("folder picker thread is not in a compatible COM apartment")
		}
		return "", false, fmt.Errorf("CoInitializeEx failed: 0x%08X", uint32(hr))
	}
	defer coUninitialize.Call()

	var dialog *comObject
	hr, _, _ = coCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if failedHRESULT(hr) || dialog == nil {
		return "", false, fmt.Errorf("CoCreateInstance failed: 0x%08X", uint32(hr))
	}
	defer releaseCOM(dialog)

	options := uintptr(fosNoChangeDir | fosPickFolders | fosForceFileSystem | fosPathMustExist)
	if hr, _, _ = syscall.SyscallN(dialog.VTable[fileDialogSetOptions], uintptr(unsafe.Pointer(dialog)), options); failedHRESULT(hr) {
		return "", false, fmt.Errorf("IFileDialog.SetOptions failed: 0x%08X", uint32(hr))
	}
	title, _ := syscall.UTF16PtrFromString("Choose FramePier download folder")
	if hr, _, _ = syscall.SyscallN(dialog.VTable[fileDialogSetTitle], uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(title))); failedHRESULT(hr) {
		return "", false, fmt.Errorf("IFileDialog.SetTitle failed: 0x%08X", uint32(hr))
	}
	setInitialFolder(dialog, currentPath)

	if owner != 0 {
		_, _, _ = setForegroundWindow.Call(owner)
		_, _, _ = bringWindowToTop.Call(owner)
	}
	hr, _, _ = syscall.SyscallN(dialog.VTable[3], uintptr(unsafe.Pointer(dialog)), owner)
	if uint32(hr) == hresultCancelled {
		return "", true, nil
	}
	if failedHRESULT(hr) {
		return "", false, fmt.Errorf("IFileDialog.Show failed: 0x%08X", uint32(hr))
	}

	var selected *comObject
	hr, _, _ = syscall.SyscallN(dialog.VTable[fileDialogGetResult], uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(&selected)))
	if failedHRESULT(hr) || selected == nil {
		return "", false, fmt.Errorf("IFileDialog.GetResult failed: 0x%08X", uint32(hr))
	}
	defer releaseCOM(selected)

	var pathPtr *uint16
	hr, _, _ = syscall.SyscallN(selected.VTable[shellItemDisplayName], uintptr(unsafe.Pointer(selected)), sigdnFileSystemPath, uintptr(unsafe.Pointer(&pathPtr)))
	if failedHRESULT(hr) || pathPtr == nil {
		return "", false, fmt.Errorf("IShellItem.GetDisplayName failed: 0x%08X", uint32(hr))
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(pathPtr)))
	path := utf16PointerString(pathPtr)
	if path == "" {
		return "", false, errors.New("folder picker returned an empty path")
	}
	return path, false, nil
}

func foregroundChromiumWindow() uintptr {
	window, _, _ := getForegroundWindow.Call()
	if window == 0 {
		return 0
	}
	root, _, _ := getAncestor.Call(window, getAncestorRoot)
	if root != 0 {
		window = root
	}
	valid, _, _ := isWindow.Call(window)
	visible, _, _ := isWindowVisible.Call(window)
	if valid == 0 || visible == 0 {
		return 0
	}
	classBuffer := make([]uint16, 128)
	length, _, _ := getClassName.Call(window, uintptr(unsafe.Pointer(&classBuffer[0])), uintptr(len(classBuffer)))
	if length == 0 || string(utf16.Decode(classBuffer[:length])) != "Chrome_WidgetWin_1" {
		return 0
	}
	var windowPID uint32
	_, _, _ = getWindowProcessID.Call(window, uintptr(unsafe.Pointer(&windowPID)))
	if windowPID == 0 || !sameWindowsSession(windowPID) {
		return 0
	}
	image, ok := processImagePath(windowPID)
	if !ok || !validChromiumExecutable(filepath.Base(image)) {
		return 0
	}
	return window
}

func validChromiumExecutable(name string) bool {
	return strings.EqualFold(name, "brave.exe") || strings.EqualFold(name, "chrome.exe") || strings.EqualFold(name, "msedge.exe")
}

func sameWindowsSession(processID uint32) bool {
	currentPID, _, _ := getCurrentProcessID.Call()
	var currentSession, targetSession uint32
	currentOK, _, _ := processIDToSession.Call(currentPID, uintptr(unsafe.Pointer(&currentSession)))
	targetOK, _, _ := processIDToSession.Call(uintptr(processID), uintptr(unsafe.Pointer(&targetSession)))
	return currentOK != 0 && targetOK != 0 && currentSession == targetSession
}

func setInitialFolder(dialog *comObject, path string) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	var item *comObject
	hr, _, _ := shCreateItem.Call(uintptr(unsafe.Pointer(pathPtr)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if failedHRESULT(hr) || item == nil {
		return
	}
	defer releaseCOM(item)
	_, _, _ = syscall.SyscallN(dialog.VTable[fileDialogSetFolder], uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(item)))
}

func releaseCOM(object *comObject) {
	if object != nil && object.VTable != nil {
		_, _, _ = syscall.SyscallN(object.VTable[2], uintptr(unsafe.Pointer(object)))
	}
}

func failedHRESULT(value uintptr) bool {
	return int32(uint32(value)) < 0
}

func utf16PointerString(value *uint16) string {
	if value == nil {
		return ""
	}
	units := unsafe.Slice(value, 32768)
	length := 0
	for length < len(units) && units[length] != 0 {
		length++
	}
	return string(utf16.Decode(units[:length]))
}
