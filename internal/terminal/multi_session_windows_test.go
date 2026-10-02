package terminal

import (
	"encoding/base64"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Exercise real ConPTY/Job Object cleanup for three shells and their children.
// This complements the deterministic fake-PTY race tests, not Wails UI acceptance.
func TestMultiSessionWindowsProcessTreeCleanup(t *testing.T) {
	var service *Service
	var mu sync.Mutex
	output := make(map[string]string)
	exits := make(chan Event, 6)
	service = New(func(name string, event Event) {
		if name == "terminal:data" {
			bytes, _ := base64.StdEncoding.DecodeString(event.Data)
			mu.Lock()
			output[event.SessionID] += string(bytes)
			mu.Unlock()
			_ = service.Ack(event.SessionID, event.Sequence)
		} else {
			exits <- event
		}
	})
	t.Cleanup(func() {
		if err := service.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	states := make([]State, 3)
	parents := make([]int, 3)
	for i := range states {
		state, err := service.Start(80, 24)
		if err != nil {
			t.Fatal(err)
		}
		states[i] = state
		p, err := service.get(state.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		parents[i] = p.pty.Pid()
		if err := service.Write(state.SessionID, "powershell -NoProfile -Command \"[Console]::WriteLine('ATLAS_CHILD_PID=' + $PID); Start-Sleep -Seconds 120\"\r\n"); err != nil {
			t.Fatal(err)
		}
	}
	pattern := regexp.MustCompile(`ATLAS_CHILD_PID=(\d+)`)
	children := make([]int, 3)
	deadline := time.Now().Add(15 * time.Second)
	for {
		found := 0
		mu.Lock()
		for i, state := range states {
			// ConPTY may insert cursor-control sequences between text chunks.
			text := regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`).ReplaceAllString(output[state.SessionID], "")
			if match := pattern.FindStringSubmatch(text); match != nil {
				children[i], _ = strconv.Atoi(match[1])
				found++
			}
		}
		mu.Unlock()
		if found == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("three child processes did not report their PIDs")
		}
		time.Sleep(20 * time.Millisecond)
	}
	openProcess := func(pid int) windows.Handle {
		t.Helper()
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if err != nil {
			t.Fatalf("open test process %d: %v", pid, err)
		}
		t.Cleanup(func() { windows.CloseHandle(handle) })
		return handle
	}
	parentHandles, childHandles := make([]windows.Handle, 3), make([]windows.Handle, 3)
	for i := range states {
		parentHandles[i], childHandles[i] = openProcess(parents[i]), openProcess(children[i])
	}
	if err := service.Stop(states[1].SessionID); err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 2} {
		for _, handle := range []windows.Handle{parentHandles[i], childHandles[i]} {
			status, err := windows.WaitForSingleObject(handle, 0)
			if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
				t.Fatalf("stopping B affected session %d", i)
			}
		}
	}
	if err := service.Shutdown(); err != nil {
		t.Fatal(err)
	}
	for i := range states {
		for _, handle := range []windows.Handle{parentHandles[i], childHandles[i]} {
			status, err := windows.WaitForSingleObject(handle, 3000)
			if err != nil || status != windows.WAIT_OBJECT_0 {
				t.Fatalf("session %d left process alive: %v/%d", i, err, status)
			}
		}
	}
	seen := make(map[string]bool)
	for range 3 {
		event := receiveEvent(t, exits)
		if seen[event.SessionID] {
			t.Fatal("duplicate real PTY exit")
		}
		seen[event.SessionID] = true
	}
	mu.Lock()
	defer mu.Unlock()
	for i, state := range states {
		if !strings.Contains(output[state.SessionID], strconv.Itoa(children[i])) {
			t.Fatal("child output routed incorrectly")
		}
	}
}
