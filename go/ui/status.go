package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

// StatusLine manages an in-place updating status line in the terminal
type StatusLine struct {
	mu          sync.Mutex
	active      bool
	message     string
	spinner     []string
	spinnerIdx  int
	stopCh      chan bool
	lastLineLen int
	isTTY       bool
}

// NewStatusLine creates a new status line manager
func NewStatusLine() *StatusLine {
	// Check if stdout is a terminal
	fileInfo, _ := os.Stdout.Stat()
	isTTY := (fileInfo.Mode() & os.ModeCharDevice) != 0

	return &StatusLine{
		spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stopCh:  make(chan bool),
		isTTY:   isTTY,
	}
}

// Show displays a static status message (no spinner)
func (s *StatusLine) Show(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isTTY {
		// For non-TTY output (logs, pipes), just print normally
		fmt.Println(msg)
		return
	}

	s.clear()
	s.message = msg
	s.active = true
	s.print(msg)
}

// ShowWithSpinner displays a status message with an animated spinner
func (s *StatusLine) ShowWithSpinner(msg string) {
	s.mu.Lock()
	s.message = msg
	s.active = true
	s.spinnerIdx = 0
	s.mu.Unlock()

	if !s.isTTY {
		// For non-TTY, just show the message once
		fmt.Println(msg)
		return
	}

	// Start spinner animation in background
	go s.animate()
}

// Update changes the message (maintains spinner if active)
func (s *StatusLine) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isTTY {
		fmt.Println(msg)
		return
	}

	// Just update the message - the spinner animation will pick it up
	s.message = msg
}

// Clear removes the status line and stops any animation
func (s *StatusLine) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active && s.isTTY {
		// Stop spinner if running
		select {
		case s.stopCh <- true:
		default:
		}
		s.clear()
	}
	s.active = false
	s.message = ""
}

// animate runs the spinner animation
func (s *StatusLine) animate() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			if !s.active {
				s.mu.Unlock()
				return
			}
			s.spinnerIdx = (s.spinnerIdx + 1) % len(s.spinner)
			s.clear()
			s.print(s.formatWithSpinner())
			s.mu.Unlock()
		}
	}
}

// print outputs text without newline
func (s *StatusLine) print(text string) {
	fmt.Print(text)
	s.lastLineLen = len(text)
}

// clear erases the current line
func (s *StatusLine) clear() {
	if s.lastLineLen > 0 {
		// Move cursor to beginning of line and clear it
		fmt.Print("\r" + strings.Repeat(" ", s.lastLineLen) + "\r")
		s.lastLineLen = 0
	}
}

// formatWithSpinner adds colored spinner to message
func (s *StatusLine) formatWithSpinner() string {
	return fmt.Sprintf("%s%s%s %s", colorGreen, s.spinner[s.spinnerIdx], colorReset, s.message)
}

// Global status line instance
var globalStatus *StatusLine
var globalStatusOnce sync.Once

// GetGlobalStatus returns the global status line instance
func GetGlobalStatus() *StatusLine {
	globalStatusOnce.Do(func() {
		globalStatus = NewStatusLine()
	})
	return globalStatus
}
