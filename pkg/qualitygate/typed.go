package qualitygate

import (
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/schema"
)

// ValidateTyped checks that data satisfies every field declared in s.
// Returns the first validation error encountered (consistent with ValidateProduces).
// Returns nil when s is empty (no constraints — pass-through, NFR40).
func ValidateTyped(data map[string]any, s schema.FieldSchema) error {
	for name, spec := range s {
		val, ok := data[name]
		if !ok {
			return fmt.Errorf("typed schema: field %q missing from output", name)
		}
		switch spec.Type {
		case "string":
			str, ok := val.(string)
			if !ok {
				return fmt.Errorf("typed schema: field %q expected string, got %T", name, val)
			}
			if len(spec.Enum) > 0 && !containsStr(spec.Enum, str) {
				return fmt.Errorf("typed schema: field %q value %q not in enum %v", name, str, spec.Enum)
			}
			if spec.MinLength > 0 && len(str) < spec.MinLength {
				return fmt.Errorf("typed schema: field %q length %d is below minimum %d", name, len(str), spec.MinLength)
			}
		case "number":
			if _, ok := val.(float64); !ok {
				return fmt.Errorf("typed schema: field %q expected number, got %T", name, val)
			}
		case "boolean":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("typed schema: field %q expected boolean, got %T", name, val)
			}
		}
	}
	return nil
}

func containsStr(enum []string, val string) bool {
	for _, e := range enum {
		if e == val {
			return true
		}
	}
	return false
}
