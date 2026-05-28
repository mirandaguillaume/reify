package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinner_StartStop(t *testing.T) {
	s := NewSpinner("testing...")
	s.Start()
	time.Sleep(200 * time.Millisecond) // let a few frames render
	s.Stop()
	// Stop is idempotent — calling again should not panic
	s.Stop()
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	// Stopping a spinner that was never started should not hang or panic.
	s := NewSpinner("never started")
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() hung on a spinner that was never started")
	}
}

func TestSpinnerChars(t *testing.T) {
	assert.Equal(t, 10, len(spinnerChars), "spinner should have 10 frames")
}
