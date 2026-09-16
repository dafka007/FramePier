package main

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

const createNoWindow = 0x08000000

var procMoveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func newHiddenCommand(name string, args ...string) *exec.Cmd {
	return hideCommandWindow(exec.Command(name, args...))
}

func newHiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return hideCommandWindow(exec.CommandContext(ctx, name, args...))
}

func hideCommandWindow(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
		HideWindow:    true,
	}
	return cmd
}

func replaceFile(source, destination string) error {
	sourcePtr, _ := syscall.UTF16PtrFromString(source)
	destinationPtr, _ := syscall.UTF16PtrFromString(destination)
	result, _, callErr := procMoveFileExW.Call(uintptr(unsafe.Pointer(sourcePtr)), uintptr(unsafe.Pointer(destinationPtr)), 0x1|0x8)
	if result == 0 {
		return fmt.Errorf("replace file: %w", callErr)
	}
	return nil
}
