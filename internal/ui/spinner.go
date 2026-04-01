package ui

import (
	"fmt"
	"time"
)

// Spinner displays an animated progress indicator in the terminal while a
// blocking operation runs. It prints in a background goroutine so the caller
// is not blocked.
type Spinner struct {
	message string
	done    chan struct{} // sending on this channel signals the goroutine to stop
}

// NewSpinner creates a Spinner with the given message displayed next to the frame.
func NewSpinner(message string) *Spinner {
	return &Spinner{message: message, done: make(chan struct{})}
}

// Start launches the spinner in a background goroutine and returns immediately.
// The goroutine loops through braille frames every 80ms until Stop() is called.
func (s *Spinner) Start() {
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.done:
				// Stop() sent on done — exit goroutine with no I/O so
				// Stop() can safely print the result line immediately after.
				return
			default:
				fmt.Printf("\r  %s %s", frames[i%len(frames)], s.message)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()
}

// Stop signals the spinner goroutine to exit, then clears the spinner line.
// All I/O after this point happens on the caller's goroutine — no race with
// the spinner goroutine since it does no I/O after receiving the stop signal.
func (s *Spinner) Stop() {
	s.done <- struct{}{}
	fmt.Printf("\r\033[K") // \033[K clears from cursor to end of line
}
