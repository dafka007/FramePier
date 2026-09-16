package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	shutdownEventName        = `Local\VidDock.Helper.Shutdown.8D75A5B3-508A-43E5-92E7-4D86C75CC8D7`
	lifecycleNoneRunning     = 0
	lifecycleIdleRunning     = 10
	lifecycleActiveRunning   = 11
	lifecycleUnknownActivity = 12
	processQueryLimitedInfo  = 0x1000
	stillActive              = 259
	waitObject0              = 0
	infiniteWait             = 0xffffffff
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW           = kernel32.NewProc("CreateEventW")
	procSetEvent               = kernel32.NewProc("SetEvent")
	procResetEvent             = kernel32.NewProc("ResetEvent")
	procWaitForSingleObject    = kernel32.NewProc("WaitForSingleObject")
	procQueryFullProcessImageW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetExitCodeProcess     = kernel32.NewProc("GetExitCodeProcess")
)

type shutdownEvent struct{ handle syscall.Handle }

type runtimeState struct {
	PID        int `json:"pid"`
	ActiveJobs int `json:"activeJobs"`
}

func openShutdownEvent() (*shutdownEvent, error) {
	target, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return openShutdownEventForTarget(target)
}

func openShutdownEventForTarget(target string) (*shutdownEvent, error) {
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(target))))
	return openNamedShutdownEvent(fmt.Sprintf("%s.%x", shutdownEventName, digest[:16]))
}

func openNamedShutdownEvent(eventName string) (*shutdownEvent, error) {
	name, _ := syscall.UTF16PtrFromString(eventName)
	handle, _, callErr := procCreateEventW.Call(0, 1, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, fmt.Errorf("create shutdown event: %w", callErr)
	}
	return &shutdownEvent{handle: syscall.Handle(handle)}, nil
}

func (event *shutdownEvent) close() {
	if event != nil && event.handle != 0 {
		_ = syscall.CloseHandle(event.handle)
		event.handle = 0
	}
}

func (event *shutdownEvent) set() error {
	result, _, callErr := procSetEvent.Call(uintptr(event.handle))
	if result == 0 {
		return fmt.Errorf("set shutdown event: %w", callErr)
	}
	return nil
}

func (event *shutdownEvent) reset() error {
	result, _, callErr := procResetEvent.Call(uintptr(event.handle))
	if result == 0 {
		return fmt.Errorf("reset shutdown event: %w", callErr)
	}
	return nil
}

func (event *shutdownEvent) signaled() bool {
	result, _, _ := procWaitForSingleObject.Call(uintptr(event.handle), 0)
	return result == waitObject0
}

func (event *shutdownEvent) wait() bool {
	result, _, _ := procWaitForSingleObject.Call(uintptr(event.handle), infiniteWait)
	return result == waitObject0
}

func lifecycleStatus(target string) (int, error) {
	target, err := validateLifecycleTarget(target)
	if err != nil {
		return 1, err
	}
	processes, err := processesAtPath(target)
	if err != nil {
		return 1, err
	}
	if len(processes) == 0 {
		return lifecycleNoneRunning, nil
	}
	unknown := false
	for _, pid := range processes {
		state, err := readRuntimeState(pid)
		if err != nil {
			unknown = true
			continue
		}
		if state.ActiveJobs > 0 {
			return lifecycleActiveRunning, nil
		}
	}
	if unknown {
		return lifecycleUnknownActivity, nil
	}
	return lifecycleIdleRunning, nil
}

func stopInstalledHelpers(target string) error {
	target, err := validateLifecycleTarget(target)
	if err != nil {
		return err
	}
	event, err := openShutdownEventForTarget(target)
	if err != nil {
		return err
	}
	defer event.close()
	if err := createLifecycleGate(target); err != nil {
		return err
	}
	if err := event.set(); err != nil {
		return err
	}
	// Older releases listen to one shared event. Only signal it when every
	// running helper we can identify belongs to the requested installation.
	// Otherwise the exact-handle fallback below safely stops only the target.
	if !otherHelperInstallationRunning(target) {
		legacy, legacyErr := openNamedShutdownEvent(shutdownEventName)
		if legacyErr == nil {
			_ = legacy.set()
			defer legacy.close()
		}
	}
	if waitForPathProcesses(target, 10*time.Second) {
		cleanupStaleRuntimeStates(target)
		if !otherHelperInstallationRunning(target) {
			cleanupStoppedJobTemps()
		}
		return nil
	}

	processes, err := processesAtPath(target)
	if err != nil {
		return err
	}
	for _, pid := range processes {
		// Revalidate using the very handle that is terminated. A recycled PID
		// must never cause another executable to be stopped. Closing the helper
		// also closes its kill-on-close jobs, including downloader descendants.
		_ = terminateHelperAtPath(pid, target)
	}
	if !waitForPathProcesses(target, 10*time.Second) {
		return errors.New("VidDockHelper.exe did not exit after graceful and forced shutdown")
	}
	cleanupStaleRuntimeStates(target)
	if !otherHelperInstallationRunning(target) {
		cleanupStoppedJobTemps()
	}
	return nil
}

func cleanupStoppedJobTemps() {
	tempRoot := filepath.Join(localAppData(), "VidDock", "temp")
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_ = removePrivateTree(filepath.Join(tempRoot, entry.Name()), 0)
		}
	}
}

func resumeInstalledHelpers() error {
	gate, _ := os.ReadFile(lifecycleGatePath())
	target := strings.TrimSpace(string(gate))
	if _, err := validateLifecycleTarget(target); err != nil {
		target, _ = os.Executable()
	}
	_ = os.Remove(lifecycleGatePath())
	event, err := openShutdownEventForTarget(target)
	if err != nil {
		return err
	}
	defer event.close()
	legacy, legacyErr := openNamedShutdownEvent(shutdownEventName)
	if legacyErr == nil {
		_ = legacy.reset()
		legacy.close()
	}
	return event.reset()
}

func lifecycleGatePath() string {
	return filepath.Join(localAppData(), "VidDock", "runtime", "installing.lock")
}

func createLifecycleGate(target string) error {
	if err := os.MkdirAll(filepath.Dir(lifecycleGatePath()), 0700); err != nil {
		return err
	}
	return os.WriteFile(lifecycleGatePath(), []byte(target), 0600)
}

func lifecycleGateActive() bool {
	info, err := os.Stat(lifecycleGatePath())
	if err != nil {
		return false
	}
	if time.Since(info.ModTime()) > 15*time.Minute {
		_ = os.Remove(lifecycleGatePath())
		return false
	}
	data, err := os.ReadFile(lifecycleGatePath())
	if err == nil {
		if target, validationErr := validateLifecycleTarget(strings.TrimSpace(string(data))); validationErr == nil {
			current, _ := os.Executable()
			return strings.EqualFold(target, filepath.Clean(current))
		}
	}
	return true
}

func otherHelperInstallationRunning(target string) bool {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return true // Unknown ownership: never broadcast or remove shared temps.
	}
	defer syscall.CloseHandle(snapshot)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	if syscall.Process32First(snapshot, &entry) != nil {
		return true
	}
	for {
		if entry.ProcessID != uint32(os.Getpid()) && strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), "VidDockHelper.exe") {
			image, ok := processImagePath(entry.ProcessID)
			if !ok || !strings.EqualFold(filepath.Clean(image), target) {
				return true
			}
		}
		if err := syscall.Process32Next(snapshot, &entry); err != nil {
			return !errors.Is(err, syscall.ERROR_NO_MORE_FILES)
		}
	}
}

func waitForPathProcesses(target string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		processes, err := processesAtPath(target)
		if err == nil && len(processes) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func validateLifecycleTarget(target string) (string, error) {
	if !filepath.IsAbs(target) || strings.HasPrefix(target, `\\`) || !strings.EqualFold(filepath.Base(target), "VidDockHelper.exe") {
		return "", errors.New("invalid VidDock lifecycle target")
	}
	target = filepath.Clean(target)
	parent := filepath.Dir(target)
	if parent == filepath.VolumeName(parent)+`\` || hasReparsePoint(parent) {
		return "", errors.New("unsafe VidDock lifecycle target")
	}
	return target, nil
}

func processesAtPath(target string) ([]uint32, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snapshot)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	if err := syscall.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}
	var matches []uint32
	for {
		if entry.ProcessID != uint32(os.Getpid()) && strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), filepath.Base(target)) {
			if image, ok := processImagePath(entry.ProcessID); ok && strings.EqualFold(filepath.Clean(image), target) {
				matches = append(matches, entry.ProcessID)
			}
		}
		if err := syscall.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return matches, nil
}

func processImagePath(pid uint32) (string, bool) {
	handle, err := syscall.OpenProcess(processQueryLimitedInfo, false, pid)
	if err != nil {
		return "", false
	}
	defer syscall.CloseHandle(handle)
	return processHandleImagePath(handle)
}

func processHandleImagePath(handle syscall.Handle) (string, bool) {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	result, _, _ := procQueryFullProcessImageW.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if result == 0 || size == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buffer[:size]), true
}

func terminateHelperAtPath(pid uint32, target string) error {
	handle, err := syscall.OpenProcess(processQueryLimitedInfo|processTerminate, false, pid)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	image, ok := processHandleImagePath(handle)
	if !ok || !strings.EqualFold(filepath.Clean(image), target) {
		return errors.New("helper process identity changed")
	}
	return syscall.TerminateProcess(handle, 1)
}

func processStillRunning(pid uint32) bool {
	handle, err := syscall.OpenProcess(processQueryLimitedInfo, false, pid)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	result, _, _ := procGetExitCodeProcess.Call(uintptr(handle), uintptr(unsafe.Pointer(&code)))
	return result != 0 && code == stillActive
}

func runtimeDirectory() string { return filepath.Join(localAppData(), "VidDock", "runtime") }

func runtimeStatePath(pid int) string {
	return filepath.Join(runtimeDirectory(), strconv.Itoa(pid)+".json")
}

func readRuntimeState(pid uint32) (runtimeState, error) {
	data, err := os.ReadFile(runtimeStatePath(int(pid)))
	if err != nil {
		return runtimeState{}, err
	}
	var state runtimeState
	if err := json.Unmarshal(data, &state); err != nil || state.PID != int(pid) {
		return runtimeState{}, errors.New("invalid runtime state")
	}
	return state, nil
}

func cleanupStaleRuntimeStates(_ string) {
	entries, err := os.ReadDir(runtimeDirectory())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		pidText := strings.TrimSuffix(entry.Name(), ".json")
		pid, err := strconv.ParseUint(pidText, 10, 32)
		if err != nil || !processStillRunning(uint32(pid)) {
			_ = os.Remove(filepath.Join(runtimeDirectory(), entry.Name()))
		}
	}
}
