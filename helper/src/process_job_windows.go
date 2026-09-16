package main

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

var (
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformationStruct struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type processJob struct {
	handle syscall.Handle
	once   sync.Once
}

func attachKillOnCloseJob(pid int) (*processJob, error) {
	handle, _, callErr := procCreateJobObjectW.Call(0, 0)
	if handle == 0 {
		return nil, fmt.Errorf("create process job: %w", callErr)
	}
	job := &processJob{handle: syscall.Handle(handle)}
	info := jobObjectExtendedLimitInformationStruct{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	result, _, callErr := procSetInformationJobObject.Call(handle, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if result == 0 {
		job.close()
		return nil, fmt.Errorf("configure process job: %w", callErr)
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		job.close()
		return nil, fmt.Errorf("open downloader process: %w", err)
	}
	defer syscall.CloseHandle(process)
	result, _, callErr = procAssignProcessToJobObject.Call(handle, uintptr(process))
	if result == 0 {
		job.close()
		return nil, fmt.Errorf("assign downloader process job: %w", callErr)
	}
	return job, nil
}

func (job *processJob) close() {
	if job == nil {
		return
	}
	job.once.Do(func() {
		if job.handle != 0 {
			_ = syscall.CloseHandle(job.handle)
			job.handle = 0
		}
	})
}
