package claude

import "github.com/mirandaguillaume/reify/pkg/model"

// EffortToModel maps an EffortLevel to a Claude model name.
// Empty effort defaults to "sonnet" (medium).
func EffortToModel(effort model.EffortLevel) string {
	switch effort {
	case model.EffortLight:
		return "haiku"
	case model.EffortHeavy:
		return "opus"
	default:
		return "sonnet"
	}
}
