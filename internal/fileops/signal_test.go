package fileops

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithCleanup_ParentCancel(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	ctx, cancel := WithCleanup(parent)
	defer cancel()

	// Cancel parent — child should also be cancelled
	parentCancel()
	<-ctx.Done()
	assert.Error(t, ctx.Err())
}

func TestWithCleanup_DirectCancel(t *testing.T) {
	ctx, cancel := WithCleanup(context.Background())
	cancel()
	<-ctx.Done()
	assert.Error(t, ctx.Err())
}
