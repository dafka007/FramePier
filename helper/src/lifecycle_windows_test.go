package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestValidateLifecycleTargetIsRestrictedToInstalledHelperName(t *testing.T) {
	valid := filepath.Join(t.TempDir(), "VidDockHelper.exe")
	if got, err := validateLifecycleTarget(valid); err != nil || got != filepath.Clean(valid) {
		t.Fatalf("valid target = %q, %v", got, err)
	}
	invalid := []string{"VidDockHelper.exe", `\\server\share\VidDockHelper.exe`, `C:\VidDockHelper.exe`, filepath.Join(t.TempDir(), "ffmpeg.exe")}
	for _, target := range invalid {
		if _, err := validateLifecycleTarget(target); err == nil {
			t.Errorf("accepted unsafe lifecycle target %q", target)
		}
	}
}

func TestKillOnCloseJobTerminatesOwnedProcess(t *testing.T) {
	cmd := newHiddenCommand("cmd.exe", "/d", "/c", "ping -n 30 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := attachKillOnCloseJob(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}
	job.close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("owned process survived closing its Job Object")
	}
}

func TestRuntimeStateRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	path := runtimeStatePath(os.Getpid())
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	data := []byte(`{"pid":` + strconv.Itoa(os.Getpid()) + `,"activeJobs":1}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	state, err := readRuntimeState(uint32(os.Getpid()))
	if err != nil || state.ActiveJobs != 1 {
		t.Fatalf("state = %#v, %v", state, err)
	}
}

func TestShutdownEventsAreInstallationScoped(t *testing.T) {
	root := t.TempDir()
	first, err := openShutdownEventForTarget(filepath.Join(root, "first", "VidDockHelper.exe"))
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	second, err := openShutdownEventForTarget(filepath.Join(root, "second", "VidDockHelper.exe"))
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	if err := first.set(); err != nil {
		t.Fatal(err)
	}
	if !first.signaled() || second.signaled() {
		t.Fatal("shutdown event reached another installation")
	}
}

func TestTerminationRejectsMismatchedProcessHandle(t *testing.T) {
	if err := terminateHelperAtPath(uint32(os.Getpid()), filepath.Join(t.TempDir(), "VidDockHelper.exe")); err == nil {
		t.Fatal("mismatched process identity was accepted")
	}
}
