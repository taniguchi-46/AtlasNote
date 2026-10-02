package terminal

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kodecable/crosspty"
)

const (
	minSize      = 1
	maxSize      = 500
	maxInput     = 16 * 1024
	drainTimeout = 30 * time.Second
)

type Event struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data,omitempty"` // Base64 preserves UTF-8 sequences split across reads.
	Sequence  uint64 `json:"sequence,omitempty"`
	ExitCode  int    `json:"exitCode"`
	Stopped   bool   `json:"stopped,omitempty"`
}

type State struct {
	SessionID string `json:"sessionId"`
}

var (
	ErrSessionEnded        = errors.New("terminal session has ended")
	ErrSessionNotFound     = errors.New("terminal session not found")
	ErrServiceShuttingDown = errors.New("terminal service is shutting down")
)

type Service struct {
	mu           sync.Mutex
	sessions     map[string]*session
	ended        map[string]<-chan struct{}
	starts       sync.WaitGroup
	shuttingDown bool
	emit         func(string, Event)
	start        func(crosspty.CommandConfig) (crosspty.Pty, error)
}

type session struct {
	id        string
	pty       crosspty.Pty
	mu        sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	closed    bool
	stopped   bool
	done      chan struct{}
	ack       chan uint64
	ended     chan struct{}
}

func New(emit func(string, Event)) *Service {
	return &Service{emit: emit, start: crosspty.Start, sessions: make(map[string]*session), ended: make(map[string]<-chan struct{})}
}

func validSize(cols, rows int) bool {
	return cols >= minSize && cols <= maxSize && rows >= minSize && rows <= maxSize
}

func (s *Service) Start(cols, rows int) (State, error) {
	if !validSize(cols, rows) {
		return State{}, errors.New("invalid terminal size")
	}
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return State{}, ErrServiceShuttingDown
	}
	// Add under the same lock that prohibits new starts during Shutdown.
	s.starts.Add(1)
	s.mu.Unlock()
	defer s.starts.Done()
	config, err := shellConfig(cols, rows)
	if err != nil {
		return State{}, errors.New("terminal shell is unavailable")
	}
	p, err := s.start(config)
	if err != nil {
		return State{}, errors.New("terminal shell could not start")
	}
	id, err := newSessionID()
	if err != nil {
		_ = p.Close()
		return State{}, errors.New("terminal session could not start")
	}
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		_ = p.Close()
		return State{}, ErrServiceShuttingDown
	}
	session := &session{id: id, pty: p, done: make(chan struct{}), ack: make(chan uint64, 1), ended: make(chan struct{})}
	s.sessions[id] = session
	go s.run(session)
	s.mu.Unlock()
	return State{SessionID: id}, nil
}

func (s *Service) get(id string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.sessions[id]
	if id == "" || p == nil {
		if _, ok := s.ended[id]; ok {
			return nil, ErrSessionEnded
		}
		return nil, ErrSessionNotFound
	}
	return p, nil
}

func (s *Service) Write(id, data string) error {
	if len(data) == 0 || len(data) > maxInput {
		return errors.New("invalid terminal input size")
	}
	p, err := s.get(id)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrSessionEnded
	}
	p.mu.Unlock()
	_, err = io.WriteString(p.pty, data)
	return err
}

func (s *Service) Resize(id string, cols, rows int) error {
	if !validSize(cols, rows) {
		return errors.New("invalid terminal size")
	}
	p, err := s.get(id)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrSessionEnded
	}
	p.mu.Unlock()
	return p.pty.Resize(crosspty.TermSize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (s *Service) Ack(id string, sequence uint64) error {
	p, err := s.get(id)
	if err != nil {
		return err
	}
	select {
	case p.ack <- sequence:
	default:
	}
	return nil
}

func (s *Service) Stop(id string) error {
	p, err := s.get(id)
	if err != nil {
		s.mu.Lock()
		ended := s.ended[id]
		s.mu.Unlock()
		if ended != nil {
			return waitEnded(ended)
		}
		return err
	}
	closeErr := p.stop()
	return errors.Join(closeErr, waitEnded(p.ended))
}

func (s *Service) Shutdown() error {
	s.mu.Lock()
	s.shuttingDown = true
	active := make([]*session, 0, len(s.sessions))
	for _, p := range s.sessions {
		active = append(active, p)
	}
	ended := make([]<-chan struct{}, 0, len(s.ended))
	for _, done := range s.ended {
		ended = append(ended, done)
	}
	s.mu.Unlock()
	// Close every PTY independently: a slow Close or in-flight Start must not
	// prevent other sessions from releasing their process trees and ACK waiters.
	results := make(chan error, len(active)+len(ended))
	for _, p := range active {
		go func() { results <- errors.Join(p.stop(), waitEnded(p.ended)) }()
	}
	for _, done := range ended {
		go func() { results <- waitEnded(done) }()
	}
	// Starts that finish after the shutdown flag close their newly created PTY
	// before Done; none can be published into the active map after the snapshot.
	s.starts.Wait()
	var err error
	for range len(active) + len(ended) {
		err = errors.Join(err, <-results)
	}
	return err
}

func waitEnded(ended <-chan struct{}) error {
	select {
	case <-ended:
		return nil
	case <-time.After(6 * time.Second):
		return errors.New("terminal shutdown timed out")
	}
}

func (p *session) stop() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.stopped = true
		close(p.done)
	}
	p.mu.Unlock()
	return p.close()
}

func (p *session) close() error {
	p.closeOnce.Do(func() { p.closeErr = p.pty.Close() })
	return p.closeErr
}

func (s *Service) run(p *session) {
	chunks := make(chan []byte, 8)
	readDone := make(chan error, 1)
	go func() {
		defer close(chunks)
		buf := make([]byte, 4096)
		for {
			n, err := p.pty.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				select {
				case chunks <- chunk:
				case <-p.done:
					readDone <- nil
					return
				}
			}
			if err != nil {
				readDone <- err
				return
			}
		}
	}()
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		var sequence uint64
		for chunk := range chunks {
			select {
			case <-p.done:
				return
			default:
			}
			sequence++
			s.emit("terminal:data", Event{SessionID: p.id, Data: base64.StdEncoding.EncodeToString(chunk), Sequence: sequence})
			for {
				select {
				case ack := <-p.ack:
					if ack == sequence {
						goto next
					}
				case <-p.done:
					return
				}
			}
		next:
		}
	}()
	waitDone := make(chan int, 1)
	go func() { waitDone <- p.pty.Wait() }()
	var code int
	readerFinished := false
	select {
	case code = <-waitDone:
	case <-readDone:
		readerFinished = true
		// EOF and other read failures both end the output stream. A child
		// that has not exited must be stopped before waiting for it.
		select {
		case code = <-waitDone:
		default:
			_ = p.stop()
			code = <-waitDone
		}
	}
	if !readerFinished {
		select {
		case <-readDone:
		case <-time.After(drainTimeout):
			p.stop()
		}
	}
	select {
	case <-senderDone:
	case <-time.After(drainTimeout):
		p.stop()
	}
	_ = p.close()
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.done)
	}
	stopped := p.stopped
	p.mu.Unlock()
	s.mu.Lock()
	delete(s.sessions, p.id)
	// Retain only the ID and completion signal for ended-ID errors and
	// idempotent Stop, including the window before the exit event finishes.
	s.ended[p.id] = p.ended
	s.mu.Unlock()
	s.emit("terminal:exit", Event{SessionID: p.id, ExitCode: code, Stopped: stopped})
	close(p.ended)
}

func shellConfig(cols, rows int) (crosspty.CommandConfig, error) {
	shell := defaultShell()
	if shell == "" {
		return crosspty.CommandConfig{}, fmt.Errorf("shell unavailable")
	}
	dir, err := os.UserHomeDir()
	if err != nil || dir == "" {
		dir, _ = os.Getwd()
	}
	env := os.Environ()
	if executable, err := os.Executable(); err == nil {
		path := os.Getenv("PATH")
		dirOfExecutable := filepath.Dir(executable)
		if dirOfExecutable != "" {
			env = setPath(env, dirOfExecutable+string(os.PathListSeparator)+path)
		}
	}
	return crosspty.CommandConfig{
		Argv: []string{shell}, Dir: dir, Env: env,
		Size:        crosspty.TermSize{Cols: uint16(cols), Rows: uint16(rows)},
		CloseConfig: crosspty.CloseConfig{CloseTimeout: 5 * time.Second, KillDelay: 2 * time.Second, KillMode: crosspty.KillModeKillGroupOnSubProcessExit},
	}, nil
}

func setPath(env []string, value string) []string {
	for i, entry := range env {
		if strings.EqualFold(strings.SplitN(entry, "=", 2)[0], "PATH") {
			env[i] = "PATH=" + value
			return env
		}
	}
	return append(env, "PATH="+value)
}
