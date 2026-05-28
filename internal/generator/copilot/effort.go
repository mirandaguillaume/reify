package copilot

import "github.com/mirandaguillaume/reify/pkg/model"

// EffortToModel maps an EffortLevel to a model name for Copilot targets.
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
