package doctor

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var spinnerChars = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Spinner renders an animated progress indicator on stderr.
// Only active in TTY mode; no-ops when stopped or in pipe/CI.
type Spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	started atomic.Bool
}

// NewSpinner creates a spinner that displays the given message.
// Call Start() to begin animation and Stop() to clear and halt.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	s.started.Store(true)
	go func() {
		defer close(s.done)
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				// Clear the spinner line
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%c %s", spinnerChars[i%len(spinnerChars)], s.message)
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the line. Safe to call even if
// Start() was never called or if called multiple times.
func (s *Spinner) Stop() {
	s.once.Do(func() {
		close(s.stop)
		if s.started.Load() {
			<-s.done
		}
	})
}
