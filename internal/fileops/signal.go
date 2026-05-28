package fileops

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithCleanup returns a context that is cancelled on SIGINT or SIGTERM.
// The caller should pass this context to SafeWrite so that it can abort
// cleanly without partial writes if the user presses Ctrl+C.
func WithCleanup(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	return ctx, cancel
}
