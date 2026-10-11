//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// The launcher is suspended until it belongs to a kill-on-close job. Assigning
// after an ordinary Start allows uv to create an unowned Python descendant.
const createSuspended = 0x00000004

var (
	createJobObject      = kernel32.NewProc("CreateJobObjectW")
	setJobInformation    = kernel32.NewProc("SetInformationJobObject")
	assignJobProcess     = kernel32.NewProc("AssignProcessToJobObject")
	openProcess          = kernel32.NewProc("OpenProcess")
	createThreadSnapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	threadFirst          = kernel32.NewProc("Thread32First")
	threadNext           = kernel32.NewProc("Thread32Next")
	openThread           = kernel32.NewProc("OpenThread")
	resumeThread         = kernel32.NewProc("ResumeThread")
)

type jobLimits struct {
	PerProcess, PerJob             int64
	Flags                          uint32
	MinWorkingSet, MaxWorkingSet   uintptr
	ActiveProcesses                uint32
	Affinity                       uintptr
	PriorityClass, SchedulingClass uint32
}

type extendedJobLimits struct {
	Basic                                                      jobLimits
	IO                                                         [6]uint64
	ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory uintptr
}

type threadEntry struct {
	Size, Usage, ID, OwnerPID   uint32
	BasePriority, DeltaPriority int32
	Flags                       uint32
}

type voiceProcessJob struct {
	handle syscall.Handle
	once   sync.Once
}

func (j *voiceProcessJob) close() {
	j.once.Do(func() { _ = syscall.CloseHandle(j.handle) })
}

func startVoiceProcess(cmd *exec.Cmd) (*voiceProcessJob, error) {
	h, _, err := createJobObject.Call(0, 0)
	if h == 0 {
		return nil, fmt.Errorf("create voice job: %w", err)
	}
	job := &voiceProcessJob{handle: syscall.Handle(h)}
	limits := extendedJobLimits{}
	limits.Basic.Flags = 0x2000 // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	ok, _, err := setJobInformation.Call(h, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		job.close()
		return nil, fmt.Errorf("configure voice job: %w", err)
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createSuspended
	if err := cmd.Start(); err != nil {
		job.close()
		return nil, err
	}
	fail := func(err error) (*voiceProcessJob, error) {
		_ = cmd.Process.Kill()
		job.close()
		_ = cmd.Wait()
		return nil, err
	}
	// PROCESS_SET_QUOTA | PROCESS_TERMINATE, required for job assignment.
	process, _, err := openProcess.Call(0x0101, 0, uintptr(cmd.Process.Pid))
	if process == 0 {
		return fail(fmt.Errorf("open voice process: %w", err))
	}
	ok, _, err = assignJobProcess.Call(h, process)
	_ = syscall.CloseHandle(syscall.Handle(process))
	if ok == 0 {
		return fail(fmt.Errorf("assign voice process job: %w", err))
	}
	snapshot, _, err := createThreadSnapshot.Call(0x00000004, 0) // TH32CS_SNAPTHREAD
	if snapshot == ^uintptr(0) {
		return fail(fmt.Errorf("snapshot voice thread: %w", err))
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))
	entry := threadEntry{Size: uint32(unsafe.Sizeof(threadEntry{}))}
	ok, _, _ = threadFirst.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if entry.OwnerPID == uint32(cmd.Process.Pid) {
			thread, _, err := openThread.Call(0x0002, 0, uintptr(entry.ID)) // THREAD_SUSPEND_RESUME
			if thread == 0 {
				return fail(fmt.Errorf("open voice thread: %w", err))
			}
			previous, _, err := resumeThread.Call(thread)
			_ = syscall.CloseHandle(syscall.Handle(thread))
			if previous == 0xffffffff {
				return fail(fmt.Errorf("resume voice thread: %w", err))
			}
			return job, nil
		}
		ok, _, _ = threadNext.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return fail(fmt.Errorf("suspended voice process has no primary thread"))
}
