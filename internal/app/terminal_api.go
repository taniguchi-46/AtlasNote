package app

import (
	"atlasnote/internal/terminal"
	"errors"
)

var errTerminalUnavailable = errors.New("terminal is unavailable")

// These Wails-only methods are deliberately absent from local IPC and MCP.
func (a *App) StartTerminal(cols, rows int) (terminal.State, error) {
	if a.ctx == nil || a.terminal == nil {
		return terminal.State{}, errTerminalUnavailable
	}
	return a.terminal.Start(cols, rows)
}

func (a *App) WriteTerminal(sessionID, data string) error {
	if a.terminal == nil {
		return errTerminalUnavailable
	}
	return a.terminal.Write(sessionID, data)
}

func (a *App) ResizeTerminal(sessionID string, cols, rows int) error {
	if a.terminal == nil {
		return errTerminalUnavailable
	}
	return a.terminal.Resize(sessionID, cols, rows)
}

func (a *App) AckTerminal(sessionID string, sequence uint64) error {
	if a.terminal == nil {
		return errTerminalUnavailable
	}
	return a.terminal.Ack(sessionID, sequence)
}

func (a *App) StopTerminal(sessionID string) error {
	if a.terminal == nil {
		return errTerminalUnavailable
	}
	return a.terminal.Stop(sessionID)
}
