package sandbox

import (
	"context"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAccessMiddleware_NilPolicy(t *testing.T) {
	var handlerCalled bool
	mw := FileAccessMiddleware(nil)
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{"ok": true}, nil
	})
	out, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": "/etc/passwd"}, next)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.True(t, out["ok"].(bool))
}

func TestFileAccessMiddleware_ReadAllowed(t *testing.T) {
	var handlerCalled bool
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": "src/main.go"}, next)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestFileAccessMiddleware_ReadDenied(t *testing.T) {
	var handlerCalled bool
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": "secrets/.env"}, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file access denied")
	assert.Contains(t, err.Error(), "read")
	assert.False(t, handlerCalled)
}

func TestFileAccessMiddleware_WriteAllowed(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{WriteGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "write_file", "file_path": "src/new.go"}, next)
	require.NoError(t, err)
}

func TestFileAccessMiddleware_WriteDenied(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{WriteGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "write_file", "file_path": "etc/passwd"}, next)
	assert.Error(t, err)
}

func TestFileAccessMiddleware_DenyOverridesAllow(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{
		ReadGlobs: []string{"**"},
		DenyGlobs: []string{"**/.env"},
	})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": ".env"}, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied by glob")
}

func TestFileAccessMiddleware_DeleteChecksWriteGlobs(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{WriteGlobs: []string{"tmp/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_delete", "file_path": "tmp/cache.dat"}, next)
	require.NoError(t, err)

	_, err = mw(context.Background(), map[string]any{"tool_name": "file_delete", "file_path": "src/main.go"}, next)
	assert.Error(t, err)
}

func TestFileAccessMiddleware_ListDirectoryChecksReadGlobs(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "list_directory", "file_path": "src/pkg"}, next)
	require.NoError(t, err)
}

func TestFileAccessMiddleware_NonFileToolPassesThrough(t *testing.T) {
	var called bool
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		called = true
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "search_code"}, next)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestFileAccessMiddleware_NoToolNamePassesThrough(t *testing.T) {
	var called bool
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		called = true
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"data": "hello"}, next)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestFileAccessMiddleware_EmptyGlobsAllowAll(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{}) // no globs
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": "any/path"}, next)
	require.NoError(t, err)
	_, err = mw(context.Background(), map[string]any{"tool_name": "write_file", "file_path": "any/path"}, next)
	require.NoError(t, err)
}

func TestFileAccessMiddleware_ErrorMessageDescriptive(t *testing.T) {
	mw := FileAccessMiddleware(&FileAccessPolicy{ReadGlobs: []string{"src/**"}})
	next := dag.MiddlewareFunc(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})
	_, err := mw(context.Background(), map[string]any{"tool_name": "file_read", "file_path": "secrets/key.pem"}, next)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file access denied")
	assert.Contains(t, err.Error(), "read")
	assert.Contains(t, err.Error(), "secrets/key.pem")
}

// ── DAG Integration ──────────────────────────────────────────────────

func TestFileAccessMiddleware_DAG_BlockedNodePropagatesError(t *testing.T) {
	policy := &FileAccessPolicy{ReadGlobs: []string{"src/**"}}
	a := &dag.Node{
		ID:       "a",
		Consumes: []string{"tool_name", "file_path"},
		Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "data"}, nil
		},
		Middlewares: []dag.Middleware{FileAccessMiddleware(policy)},
	}

	d, err := dag.New(a)
	require.NoError(t, err)

	_, err = d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"tool_name": "file_read",
		"file_path": "etc/passwd",
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file access denied")
}

func TestFileAccessMiddleware_DAG_AllowedNodeExecutes(t *testing.T) {
	policy := &FileAccessPolicy{ReadGlobs: []string{"src/**"}}
	a := &dag.Node{
		ID:       "a",
		Consumes: []string{"tool_name", "file_path"},
		Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "ok"}, nil
		},
		Middlewares: []dag.Middleware{FileAccessMiddleware(policy)},
	}

	d, err := dag.New(a)
	require.NoError(t, err)

	out, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"tool_name": "file_read",
		"file_path": "src/main.go",
	}))
	require.NoError(t, err)
	assert.Equal(t, "ok", out["x"])
}
