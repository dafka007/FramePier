package main

import (
	"syscall"
	"unsafe"
)

const (
	messageBoxOK              = 0x00000000
	messageBoxIconInformation = 0x00000040
)

func showInformation(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), messageBoxOK|messageBoxIconInformation)
}
