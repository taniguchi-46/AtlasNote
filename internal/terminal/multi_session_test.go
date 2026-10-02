package terminal

import (
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Kodecable/crosspty"
)

type routingPTY struct {
	*lifecyclePTY
	output  chan []byte
	mu      sync.Mutex
	writes  []string
	resizes []crosspty.TermSize
}

func newRoutingPTY() *routingPTY {
	return &routingPTY{lifecyclePTY: newLifecyclePTY(), output: make(chan []byte, 8)}
}

func (p *routingPTY) Read(buf []byte) (int, error) {
	select {
	case data := <-p.output:
		return copy(buf, data), nil
	case err := <-p.readError:
		return 0, err
	case <-p.closed:
		return 0, io.EOF
	}
}

func (p *routingPTY) Write(data []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, string(data))
	p.mu.Unlock()
	return p.lifecyclePTY.Write(data)
}

func (p *routingPTY) Resize(size crosspty.TermSize) error {
	p.mu.Lock()
	p.resizes = append(p.resizes, size)
	p.mu.Unlock()
	return nil
}

func startThree(t *testing.T) (*Service, []*routingPTY, []State, []<-chan Event, <-chan Event) {
	t.Helper()
	ptys := []*routingPTY{newRoutingPTY(), newRoutingPTY(), newRoutingPTY()}
	data := []chan Event{make(chan Event, 8), make(chan Event, 8), make(chan Event, 8)}
	exits := make(chan Event, 8)
	var eventMu sync.Mutex
	routes := make(map[string]int)
	service := New(func(name string, event Event) {
		if name == "terminal:exit" {
			exits <- event
		} else {
			eventMu.Lock()
			index, ok := routes[event.SessionID]
			eventMu.Unlock()
			if !ok {
				t.Errorf("unknown output ID %s", event.SessionID)
				return
			}
			data[index] <- event
		}
	})
	index := 0
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) {
		p := ptys[index]
		index++
		return p, nil
	}
	t.Cleanup(func() {
		if err := service.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	states := make([]State, 3)
	seen := make(map[string]bool)
	for i := range states {
		state, err := service.Start(80, 24)
		if err != nil {
			t.Fatal(err)
		}
		if state.SessionID == "" || seen[state.SessionID] {
			t.Fatal("session IDs must be unique and nonempty")
		}
		seen[state.SessionID] = true
		states[i] = state
		eventMu.Lock()
		routes[state.SessionID] = i
		eventMu.Unlock()
	}
	return service, ptys, states, []<-chan Event{data[0], data[1], data[2]}, exits
}

func receiveEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("missing terminal event")
		return Event{}
	}
}

func TestMultiSessionWriteResizeAndAckIsolation(t *testing.T) {
	service, ptys, states, data, _ := startThree(t)
	if err := service.Write(states[0].SessionID, "only A"); err != nil {
		t.Fatal(err)
	}
	if err := service.Resize(states[0].SessionID, 100, 30); err != nil {
		t.Fatal(err)
	}
	for i, p := range ptys {
		p.mu.Lock()
		writes, resizes := len(p.writes), len(p.resizes)
		p.mu.Unlock()
		want := 0
		if i == 0 {
			want = 1
		}
		if writes != want || resizes != want {
			t.Fatalf("session %d received unrelated I/O: %d/%d", i, writes, resizes)
		}
		p.output <- []byte("first")
		p.output <- []byte("second")
		event := receiveEvent(t, data[i])
		if event.SessionID != states[i].SessionID || event.Sequence != 1 {
			t.Fatalf("wrong output route: %+v", event)
		}
		decoded, _ := base64.StdEncoding.DecodeString(event.Data)
		if string(decoded) != "first" {
			t.Fatalf("wrong output: %q", decoded)
		}
	}
	if err := service.Ack(states[1].SessionID, 1); err != nil {
		t.Fatal(err)
	}
	if event := receiveEvent(t, data[1]); event.Sequence != 2 {
		t.Fatalf("B did not advance: %+v", event)
	}
	for _, i := range []int{0, 2} {
		select {
		case event := <-data[i]:
			t.Fatalf("B ACK released session %d: %+v", i, event)
		case <-time.After(30 * time.Millisecond):
		}
		if err := service.Ack(states[i].SessionID, 1); err != nil {
			t.Fatal(err)
		}
		if event := receiveEvent(t, data[i]); event.Sequence != 2 {
			t.Fatalf("session %d did not advance", i)
		}
	}
}

func TestMultiSessionExitIsolationAndEndedIDs(t *testing.T) {
	for _, action := range []string{"stop", "eof", "read-error", "natural"} {
		t.Run(action, func(t *testing.T) {
			service, ptys, states, _, exits := startThree(t)
			a, _ := service.get(states[0].SessionID)
			switch action {
			case "stop":
				if err := service.Stop(a.id); err != nil {
					t.Fatal(err)
				}
			case "eof":
				ptys[0].readError <- io.EOF
			case "read-error":
				ptys[0].readError <- errors.New("read failed")
			case "natural":
				ptys[0].exitCode = 7
				ptys[0].finishExit()
				waitForSignal(t, ptys[0].waitReturned, "natural exit")
				ptys[0].readError <- io.EOF
			}
			waitForSignal(t, a.ended, "A ended")
			event := receiveEvent(t, exits)
			if event.SessionID != a.id || event.Stopped != (action != "natural") {
				t.Fatalf("incorrect exit: %+v", event)
			}
			if action == "natural" && event.ExitCode != 7 {
				t.Fatalf("lost exit code: %+v", event)
			}
			for _, state := range states[1:] {
				if err := service.Write(state.SessionID, "still alive"); err != nil {
					t.Fatal(err)
				}
			}
			for _, operation := range []func(string) error{
				func(id string) error { return service.Write(id, "late") },
				func(id string) error { return service.Resize(id, 80, 24) },
				func(id string) error { return service.Ack(id, 1) },
			} {
				if err := operation(a.id); !errors.Is(err, ErrSessionEnded) {
					t.Fatalf("ended ID: %v", err)
				}
				for _, id := range []string{"", "unknown"} {
					if err := operation(id); !errors.Is(err, ErrSessionNotFound) {
						t.Fatalf("unknown ID: %v", err)
					}
				}
			}
			if err := service.Stop("unknown"); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("unknown Stop: %v", err)
			}
			if err := service.Shutdown(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				receiveEvent(t, exits)
			}
			select {
			case event := <-exits:
				t.Fatalf("duplicate exit: %+v", event)
			default:
			}
			for i, p := range ptys {
				if p.closeCalls.Load() != 1 || p.waitCalls.Load() != 1 {
					t.Fatalf("session %d not cleaned exactly once", i)
				}
				if err := service.Stop(states[i].SessionID); err != nil {
					t.Fatalf("old ended ID Stop: %v", err)
				}
			}
			service.mu.Lock()
			remaining := len(service.sessions)
			service.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("retained %d active sessions", remaining)
			}
		})
	}
}

func TestMultiSessionBlockedWriteDoesNotDelayOthers(t *testing.T) {
	service, ptys, states, _, _ := startThree(t)
	ptys[0].blockWrite = true
	writeDone := make(chan error, 1)
	go func() { writeDone <- service.Write(states[0].SessionID, "blocked") }()
	waitForSignal(t, ptys[0].writeEntered, "A blocked write")
	if err := service.Write(states[1].SessionID, "B input"); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(states[0].SessionID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-writeDone:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("A write remained blocked")
	}
	if err := service.Write(states[2].SessionID, "C input"); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSessionShutdownClosesOthersBeforeSlowClose(t *testing.T) {
	service, ptys, _, _, _ := startThree(t)
	ptys[0].closeEntered = make(chan struct{})
	ptys[0].closeGate = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- service.Shutdown() }()
	waitForSignal(t, ptys[0].closeEntered, "A slow Close")
	for _, p := range ptys[1:] {
		waitForSignal(t, p.exited, "other child cleanup")
	}
	close(ptys[0].closeGate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMultipleInFlightStartsShutdown(t *testing.T) {
	service, active, _, _, _ := startThree(t)
	pending := []*lifecyclePTY{newLifecyclePTY(), newLifecyclePTY(), newLifecyclePTY()}
	entered := make(chan struct{}, 3)
	gate := make(chan struct{})
	var factoryMu sync.Mutex
	next := 0
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) {
		factoryMu.Lock()
		p := pending[next]
		next++
		factoryMu.Unlock()
		entered <- struct{}{}
		<-gate
		return p, nil
	}
	starts := make(chan error, 3)
	for range 3 {
		go func() { _, err := service.Start(80, 24); starts <- err }()
	}
	for range 3 {
		waitForSignal(t, entered, "concurrent Start")
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- service.Shutdown() }()
	// The existing children must be cleaned even while all three factories wait.
	for _, p := range active {
		waitForSignal(t, p.exited, "active child cleanup during pending starts")
	}
	if _, err := service.Start(80, 24); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatal(err)
	}
	close(gate)
	for range 3 {
		if err := <-starts; !errors.Is(err, ErrServiceShuttingDown) {
			t.Fatalf("late Start published: %v", err)
		}
	}
	if err := <-shutdown; err != nil {
		t.Fatal(err)
	}
	for _, p := range pending {
		waitForSignal(t, p.exited, "unpublished child cleanup")
		if p.closeCalls.Load() != 1 {
			t.Fatal("pending PTY not closed once")
		}
	}
}

func TestConcurrentMultiSessionStarts(t *testing.T) {
	service := New(func(string, Event) {})
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return newLifecyclePTY(), nil }
	t.Cleanup(func() {
		if err := service.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	barrier := make(chan struct{})
	states := make(chan State, 3)
	errors := make(chan error, 3)
	for range 3 {
		go func() {
			<-barrier
			state, err := service.Start(80, 24)
			states <- state
			errors <- err
		}()
	}
	close(barrier)
	seen := make(map[string]bool)
	for range 3 {
		state := <-states
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
		if state.SessionID == "" || seen[state.SessionID] {
			t.Fatal("duplicate concurrent session ID")
		}
		seen[state.SessionID] = true
		if _, err := service.get(state.SessionID); err != nil {
			t.Fatal(err)
		}
	}
}
