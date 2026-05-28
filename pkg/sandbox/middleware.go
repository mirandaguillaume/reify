package sandbox

import (
	"context"
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

// toolAction maps tool names to file access actions.
var toolAction = map[string]string{
	"file_read":      "read",
	"read_file":      "read",
	"file_write":     "write",
	"write_file":     "write",
	"file_delete":    "delete",
	"delete_file":    "delete",
	"list_directory": "read",
	"list_dir":       "read",
}

// FileAccessMiddleware returns a dag.Middleware that enforces file access policy
// on tool call inputs. It inspects the inputs map for "tool_name" and "file_path"
// keys and validates them against the policy.
//
// If the policy is nil, returns a no-op middleware (zero overhead).
func FileAccessMiddleware(policy *FileAccessPolicy) dag.Middleware {
	if policy == nil {
		return func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
			return next(ctx, inputs)
		}
	}

	return func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
		// Pre-check: validate file operation before handler runs
		if err := checkInputs(policy, inputs); err != nil {
			return nil, err
		}

		// Execute the handler
		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}

		// Post-check: validate output file paths as a secondary safety net
		if err := checkInputs(policy, out); err != nil {
			return nil, fmt.Errorf("output validation: %w", err)
		}

		return out, nil
	}
}

func checkInputs(policy *FileAccessPolicy, m map[string]any) error {
	toolName, _ := m["tool_name"].(string)
	filePath, _ := m["file_path"].(string)

	if toolName == "" || filePath == "" {
		return nil // not a file operation
	}

	action, ok := toolAction[toolName]
	if !ok {
		return nil // not a file tool this middleware handles
	}

	reason, allowed := CheckAccess(policy, filePath, action)
	if !allowed {
		return fmt.Errorf("file access denied: %s %q not permitted by policy (%s)", action, filePath, reason)
	}

	return nil
}
