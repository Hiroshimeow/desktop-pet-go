//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Actual Windows children, not a mock of process/job ownership. The helper
// substitutes only the voice backend so no microphone or private audio is used.
func TestVoiceProcessHelper(t *testing.T) {
	mode := os.Getenv("PET_VOICE_PROCESS_TEST")
	if mode == "" {
		return
	}
	if mode == "descendant" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestVoiceProcessHelper$")
	child.Env = append(os.Environ(), "PET_VOICE_PROCESS_TEST=descendant")
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	fmt.Println(child.Process.Pid)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command voiceCommand
		if json.Unmarshal(scanner.Bytes(), &command) == nil && command.Type == "crash" {
			os.Exit(9)
		}
	}
	os.Exit(0)
}

func waitVoiceTestProcess(t *testing.T, pid int) {
	t.Helper()
	handle, _, _ := openProcess.Call(0x00100000, 0, uintptr(pid)) // SYNCHRONIZE
	if handle == 0 {
		return
	} // Already reaped.
	defer syscall.CloseHandle(syscall.Handle(handle))
	wait := kernel32.NewProc("WaitForSingleObject")
	result, _, err := wait.Call(handle, 3000)
	if result != 0 {
		t.Fatalf("owned process %d survived job cleanup: result=%d err=%v", pid, result, err)
	}
}

func TestVoiceJobKillsDescendantsAndCanRestartAfterCrash(t *testing.T) {
	for attempt := 0; attempt < 3; attempt++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestVoiceProcessHelper$")
		cmd.Env = append(os.Environ(), "PET_VOICE_PROCESS_TEST=parent")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		job, err := startVoiceProcess(cmd)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(job.close)
		pidLine := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(stdout)
			if scanner.Scan() {
				pidLine <- scanner.Text()
			}
		}()
		var childPID int
		select {
		case line := <-pidLine:
			childPID, err = strconv.Atoi(line)
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("job-owned launcher did not resume")
		}
		if attempt == 1 {
			if err := json.NewEncoder(stdin).Encode(voiceCommand{Type: "crash"}); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("expected helper crash")
			}
			job.close()
		} else {
			job.close()
			_ = cmd.Wait()
		}
		_ = stdin.Close()
		waitVoiceTestProcess(t, cmd.Process.Pid)
		waitVoiceTestProcess(t, childPID)
	}
}

func TestResidentVoiceCommandsAreBoundedAndNeverBlockCaller(t *testing.T) {
	controller := newVoiceController(make(chan voiceEvent), false, false, nil)
	defer controller.cancel()
	for i := 0; i < cap(controller.commands); i++ {
		if err := controller.send(voiceCommand{Type: "dictation_cancel"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := controller.send(voiceCommand{Type: "dictation_cancel"}); err == nil {
		t.Fatal("full command pipe backlog was silently accepted")
	}
	select {
	case <-controller.processContext.Done():
	default:
		t.Fatal("overloaded process retained an ambiguous audio owner")
	}
}
