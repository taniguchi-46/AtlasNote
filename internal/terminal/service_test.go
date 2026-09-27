package terminal

import (
	"encoding/base64"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kodecable/crosspty"
)

func TestTerminalLifecycle(t *testing.T) {
	var outputMu sync.Mutex
	var output strings.Builder
	exits := make(chan Event, 2)
	var service *Service
	service = New(func(name string, event Event) {
		switch name {
		case "terminal:data":
			decoded, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				t.Errorf("decode output: %v", err)
			}
			outputMu.Lock()
			output.Write(decoded)
			outputMu.Unlock()
			_ = service.Ack(event.SessionID, event.Sequence)
		case "terminal:exit":
			exits <- event
		}
	})
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(80, 24); err == nil {
		t.Fatal("second session started")
	}
	if err := service.Resize(state.SessionID, 100, 30); err != nil {
		t.Fatal(err)
	}
	if err := service.Write(state.SessionID, "echo ATLAS_TERMINAL_SMOKE 日本語\r\nexit\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-exits:
		if event.SessionID != state.SessionID || event.ExitCode != 0 || event.Stopped {
			t.Fatalf("unexpected exit: %+v", event)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("terminal did not exit")
	}
	outputMu.Lock()
	got := output.String()
	outputMu.Unlock()
	if !strings.Contains(got, "ATLAS_TERMINAL_SMOKE") {
		t.Fatalf("output missing smoke marker: %q", got)
	}
	if !strings.Contains(got, "日本語") {
		t.Fatalf("Unicode output missing: %q", got)
	}
	if err := service.Stop(state.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(state.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(80, 24); err != nil {
		t.Fatalf("restart: %v", err)
	}
	service.Shutdown()
}

func TestTerminalValidationAndStartFailure(t *testing.T) {
	service := New(func(string, Event) {})
	for _, size := range [][2]int{{0, 24}, {80, 0}, {501, 24}, {-1, 24}} {
		if _, err := service.Start(size[0], size[1]); err == nil {
			t.Fatalf("accepted invalid size %v", size)
		}
	}
	if err := service.Write("invalid", "echo"); err == nil {
		t.Fatal("accepted invalid session")
	}
	if err := service.Resize("invalid", 80, 24); err == nil {
		t.Fatal("accepted invalid session")
	}
	if err := service.Stop("invalid"); err == nil {
		t.Fatal("accepted invalid session")
	}
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return nil, errors.New("private path") }
	if _, err := service.Start(80, 24); err == nil || strings.Contains(err.Error(), "private path") {
		t.Fatalf("unsafe start error: %v", err)
	}
}

func TestTerminalStopRace(t *testing.T) {
	var service *Service
	service = New(func(name string, event Event) {
		if name == "terminal:data" {
			_ = serviceAck(service, event)
		}
	})
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = service.Stop(state.SessionID) }()
	}
	wg.Wait()
	service.Shutdown()
}

func serviceAck(service *Service, event Event) error {
	return service.Ack(event.SessionID, event.Sequence)
}

func TestTerminalExitCodeAndLargeOutput(t *testing.T) {
	var service *Service
	var total int
	var mu sync.Mutex
	exits := make(chan Event, 1)
	service = New(func(name string, event Event) {
		if name == "terminal:data" {
			decoded, _ := base64.StdEncoding.DecodeString(event.Data)
			mu.Lock()
			total += len(decoded)
			mu.Unlock()
			_ = service.Ack(event.SessionID, event.Sequence)
		} else if name == "terminal:exit" {
			exits <- event
		}
	})
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	command := "for /L %i in (1,1,2000) do @echo ATLAS_OUTPUT_LINE_%i\r\nexit 7\r\n"
	if runtime.GOOS != "windows" {
		command = "seq 1 2000\nexit 7\n"
	}
	if err := service.Write(state.SessionID, command); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-exits:
		if event.ExitCode != 7 {
			t.Fatalf("exit code = %d", event.ExitCode)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("large output did not finish")
	}
	mu.Lock()
	got := total
	mu.Unlock()
	if got < 8000 {
		t.Fatalf("large output truncated: %d bytes", got)
	}
}

func TestTerminalStopWithUnacknowledgedOutput(t *testing.T) {
	outputStarted := make(chan struct{}, 1)
	service := New(func(name string, _ Event) {
		if name == "terminal:data" {
			select {
			case outputStarted <- struct{}{}:
			default:
			}
		}
	})
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	command := "echo WAITING_FOR_ACK\r\n"
	if runtime.GOOS != "windows" {
		command = "echo WAITING_FOR_ACK\n"
	}
	if err := service.Write(state.SessionID, command); err != nil {
		t.Fatal(err)
	}
	select {
	case <-outputStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("no PTY output")
	}
	if err := service.Stop(state.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(state.SessionID); err != nil {
		t.Fatal(err)
	}
}

type lifecyclePTY struct {
	closed       chan struct{}
	exited       chan struct{}
	readEntered  chan struct{}
	waitEntered  chan struct{}
	waitReturned chan struct{}
	writeEntered chan struct{}
	readError    chan error
	closeGate    chan struct{}
	closeEntered chan struct{}
	blockWrite   bool
	closeOnce    sync.Once
	exitOnce     sync.Once
	closeCalls   atomic.Int32
	waitCalls    atomic.Int32
	closedAlive  atomic.Int32
	exitCode     int
}

func newLifecyclePTY() *lifecyclePTY {
	return &lifecyclePTY{
		closed: make(chan struct{}), exited: make(chan struct{}),
		readEntered: make(chan struct{}, 1), waitEntered: make(chan struct{}, 1), waitReturned: make(chan struct{}), writeEntered: make(chan struct{}, 1),
		readError: make(chan error, 1),
	}
}

func (p *lifecyclePTY) Read([]byte) (int, error) {
	select {
	case p.readEntered <- struct{}{}:
	default:
	}
	select {
	case err := <-p.readError:
		return 0, err
	case <-p.closed:
		return 0, io.EOF
	}
}

func (p *lifecyclePTY) Write(data []byte) (int, error) {
	if !p.blockWrite {
		return len(data), nil
	}
	select {
	case p.writeEntered <- struct{}{}:
	default:
	}
	<-p.closed
	return 0, io.ErrClosedPipe
}

func (p *lifecyclePTY) Resize(crosspty.TermSize) error { return nil }
func (p *lifecyclePTY) Pid() int                       { return 1 }
func (p *lifecyclePTY) Wait() int {
	p.waitCalls.Add(1)
	select {
	case p.waitEntered <- struct{}{}:
	default:
	}
	<-p.exited
	close(p.waitReturned)
	return p.exitCode
}
func (p *lifecyclePTY) finishExit() { p.exitOnce.Do(func() { close(p.exited) }) }
func (p *lifecyclePTY) Close() error {
	p.closeCalls.Add(1)
	p.closeOnce.Do(func() {
		select {
		case <-p.exited:
		default:
			p.closedAlive.Add(1)
		}
		if p.closeEntered != nil {
			close(p.closeEntered)
			<-p.closeGate
		}
		close(p.closed)
		p.finishExit()
	})
	return nil
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestBlockedWriteAllowsStopAndShutdown(t *testing.T) {
	for _, action := range []string{"stop", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			pty := newLifecyclePTY()
			pty.blockWrite = true
			service := New(func(string, Event) {})
			service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return pty, nil }
			state, err := service.Start(80, 24)
			if err != nil {
				t.Fatal(err)
			}
			writeDone := make(chan error, 1)
			go func() { writeDone <- service.Write(state.SessionID, "blocked input") }()
			waitForSignal(t, pty.writeEntered, "blocked PTY write")
			queuedWriteDone := make(chan error, 1)
			go func() { queuedWriteDone <- service.Write(state.SessionID, "queued input") }()
			stopDone := make(chan error, 1)
			go func() {
				if action == "stop" {
					stopDone <- service.Stop(state.SessionID)
				} else {
					stopDone <- service.Shutdown()
				}
			}()
			select {
			case err := <-stopDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stop waited for blocked write")
			}
			select {
			case err := <-writeDone:
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("blocked write was not interrupted: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("blocked write remained after PTY close")
			}
			select {
			case err := <-queuedWriteDone:
				if !errors.Is(err, ErrSessionEnded) {
					t.Fatalf("queued write reached closed PTY: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("queued write remained after PTY close")
			}
			waitForSignal(t, pty.exited, "child exit")
			if got := pty.closeCalls.Load(); got != 1 {
				t.Fatalf("PTY closed %d times", got)
			}
			if err := service.Write(state.SessionID, "late input"); !errors.Is(err, ErrSessionEnded) {
				t.Fatalf("closed session write: %v", err)
			}
			if err := service.Resize(state.SessionID, 80, 24); !errors.Is(err, ErrSessionEnded) {
				t.Fatalf("closed session resize: %v", err)
			}
		})
	}
}

func TestShutdownRejectsStartBeforeAndAfterCompletion(t *testing.T) {
	pty := newLifecyclePTY()
	pty.closeEntered = make(chan struct{})
	pty.closeGate = make(chan struct{})
	service := New(func(string, Event) {})
	var starts atomic.Int32
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) {
		starts.Add(1)
		return pty, nil
	}
	if _, err := service.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- service.Shutdown() }()
	waitForSignal(t, pty.closeEntered, "shutdown PTY close")
	if _, err := service.Start(80, 24); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatalf("start during shutdown: %v", err)
	}
	close(pty.closeGate)
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if _, err := service.Start(80, 24); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatalf("start after shutdown: %v", err)
	}
	if err := service.Shutdown(); err != nil {
		t.Fatalf("duplicate shutdown: %v", err)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("started %d PTYs", got)
	}
	if got := pty.closeCalls.Load(); got != 1 {
		t.Fatalf("PTY closed %d times", got)
	}
}

func TestEmptyShutdownRejectsStartWithoutCreatingPTY(t *testing.T) {
	service := New(func(string, Event) {})
	var starts atomic.Int32
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) {
		starts.Add(1)
		return newLifecyclePTY(), nil
	}
	if err := service.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(80, 24); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatalf("start after empty shutdown: %v", err)
	}
	if err := service.Shutdown(); err != nil {
		t.Fatalf("duplicate empty shutdown: %v", err)
	}
	if got := starts.Load(); got != 0 {
		t.Fatalf("created %d PTYs after shutdown", got)
	}
}

func TestShutdownRacesInFlightStart(t *testing.T) {
	pty := newLifecyclePTY()
	startEntered := make(chan struct{})
	allowStart := make(chan struct{})
	service := New(func(string, Event) {})
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) {
		close(startEntered)
		<-allowStart
		return pty, nil
	}
	startDone := make(chan error, 1)
	go func() { _, err := service.Start(80, 24); startDone <- err }()
	waitForSignal(t, startEntered, "in-flight PTY start")
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- service.Shutdown() }()
	deadline := time.After(3 * time.Second)
	for {
		service.mu.Lock()
		shuttingDown := service.shuttingDown
		service.mu.Unlock()
		if shuttingDown {
			break
		}
		select {
		case <-deadline:
			t.Fatal("shutdown state was not established during in-flight start")
		default:
			runtime.Gosched()
		}
	}
	close(allowStart)
	select {
	case err := <-startDone:
		if !errors.Is(err, ErrServiceShuttingDown) {
			t.Fatalf("in-flight start: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("start remained blocked")
	}
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not collect in-flight start")
	}
	waitForSignal(t, pty.exited, "child exit after start/shutdown race")
	if _, err := service.Start(80, 24); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatalf("start after raced shutdown: %v", err)
	}
}

func TestReadFailureStopsLiveChildOnce(t *testing.T) {
	pty := newLifecyclePTY()
	exits := make(chan Event, 2)
	service := New(func(name string, event Event) {
		if name == "terminal:exit" {
			exits <- event
		}
	})
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return pty, nil }
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.get(state.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, pty.readEntered, "PTY read")
	pty.readError <- errors.New("read failed while child is alive")
	waitForSignal(t, session.ended, "session end after read failure")
	waitForSignal(t, pty.exited, "child exit after read failure")
	select {
	case event := <-exits:
		if event.SessionID != state.SessionID || !event.Stopped {
			t.Fatalf("unexpected exit event: %+v", event)
		}
	default:
		t.Fatal("missing exit event")
	}
	select {
	case event := <-exits:
		t.Fatalf("duplicate exit event: %+v", event)
	default:
	}
	if got := pty.closeCalls.Load(); got != 1 {
		t.Fatalf("PTY closed %d times", got)
	}
}

func TestStopExitAndReadFailureRaceCleansUpOnce(t *testing.T) {
	pty := newLifecyclePTY()
	exits := make(chan Event, 2)
	service := New(func(name string, event Event) {
		if name == "terminal:exit" {
			exits <- event
		}
	})
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return pty, nil }
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.get(state.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, pty.readEntered, "PTY read")
	barrier := make(chan struct{})
	var workers sync.WaitGroup
	for _, action := range []func(){
		func() { _ = service.Stop(state.SessionID) },
		func() { pty.finishExit() },
		func() { pty.readError <- errors.New("read failure") },
	} {
		workers.Add(1)
		go func(action func()) { defer workers.Done(); <-barrier; action() }(action)
	}
	close(barrier)
	workersDone := make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	waitForSignal(t, workersDone, "stop/exit/read race workers")
	waitForSignal(t, session.ended, "session end after stop/exit/read race")
	if got := pty.closeCalls.Load(); got != 1 {
		t.Fatalf("PTY closed %d times", got)
	}
	select {
	case <-exits:
	default:
		t.Fatal("missing exit event")
	}
	select {
	case event := <-exits:
		t.Fatalf("duplicate exit event: %+v", event)
	default:
	}
}

func startFakeTerminal(t *testing.T, pty *lifecyclePTY) (*Service, State, *session, <-chan Event) {
	t.Helper()
	exits := make(chan Event, 2)
	service := New(func(name string, event Event) {
		if name == "terminal:exit" {
			exits <- event
		}
	})
	service.start = func(crosspty.CommandConfig) (crosspty.Pty, error) { return pty, nil }
	state, err := service.Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.get(state.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, pty.readEntered, "PTY read")
	waitForSignal(t, pty.waitEntered, "blocked process Wait")
	return service, state, session, exits
}

func assertSingleFakeExit(t *testing.T, pty *lifecyclePTY, session *session, exits <-chan Event, state State, stopped bool, exitCode int) {
	t.Helper()
	waitForSignal(t, session.ended, "terminal session end")
	waitForSignal(t, pty.exited, "child process exit")
	waitForSignal(t, pty.waitReturned, "process Wait collection")
	select {
	case event := <-exits:
		if event.SessionID != state.SessionID || event.Stopped != stopped || event.ExitCode != exitCode {
			t.Fatalf("unexpected exit event: %+v", event)
		}
	default:
		t.Fatal("missing exit event")
	}
	select {
	case event := <-exits:
		t.Fatalf("duplicate exit event: %+v", event)
	default:
	}
	if got := pty.closeCalls.Load(); got != 1 {
		t.Fatalf("PTY closed %d times", got)
	}
	if got := pty.waitCalls.Load(); got != 1 {
		t.Fatalf("process waited %d times", got)
	}
}

func TestReadEOFFirstStopsLiveChild(t *testing.T) {
	pty := newLifecyclePTY()
	service, state, session, exits := startFakeTerminal(t, pty)
	select {
	case <-pty.exited:
		t.Fatal("child exited before reader EOF")
	default:
	}
	pty.readError <- io.EOF
	assertSingleFakeExit(t, pty, session, exits, state, true, 0)
	if got := pty.closedAlive.Load(); got != 1 {
		t.Fatalf("live child was not stopped: %d", got)
	}
	if err := service.Stop(state.SessionID); err != nil {
		t.Fatal(err)
	}
}

func TestNaturalExitBeforeReadEOFKeepsExitCode(t *testing.T) {
	pty := newLifecyclePTY()
	pty.exitCode = 7
	_, state, session, exits := startFakeTerminal(t, pty)
	pty.finishExit()
	waitForSignal(t, pty.waitReturned, "natural process exit")
	pty.readError <- io.EOF
	assertSingleFakeExit(t, pty, session, exits, state, false, 7)
	if got := pty.closedAlive.Load(); got != 0 {
		t.Fatalf("already exited child was killed: %d", got)
	}
}

func TestReadEOFRacesStopAndShutdown(t *testing.T) {
	for _, action := range []string{"stop", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			pty := newLifecyclePTY()
			service, state, session, exits := startFakeTerminal(t, pty)
			barrier := make(chan struct{})
			operationDone := make(chan error, 1)
			go func() {
				<-barrier
				if action == "stop" {
					operationDone <- service.Stop(state.SessionID)
				} else {
					operationDone <- service.Shutdown()
				}
			}()
			go func() { <-barrier; pty.readError <- io.EOF }()
			close(barrier)
			select {
			case err := <-operationDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stop/shutdown blocked after EOF")
			}
			assertSingleFakeExit(t, pty, session, exits, state, true, 0)
		})
	}
}
