package claude_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/claude"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestEffortToModel_Light(t *testing.T) {
	assert.Equal(t, "haiku", claude.EffortToModel(model.EffortLight))
}

func TestEffortToModel_Medium(t *testing.T) {
	assert.Equal(t, "sonnet", claude.EffortToModel(model.EffortMedium))
}

func TestEffortToModel_Heavy(t *testing.T) {
	assert.Equal(t, "opus", claude.EffortToModel(model.EffortHeavy))
}

func TestEffortToModel_Empty(t *testing.T) {
	assert.Equal(t, "sonnet", claude.EffortToModel(""))
}
