package doctor

import (
	"bytes"
	"os"
	"testing"
)

// The doctor pipeline specs live in two places: embedded here (go:embed can
// only reach package-local files, so the command needs a local copy) and
// mirrored into the repo's top-level skills/ + agents/ dogfood set, which
// `reify build` and `reify score` consume. The two copies must stay identical;
// this guards against drift — the exact gap that let the top-level skills go
// missing (the agent was copied, its skills were not).
func TestDoctorSpecs_MirrorTopLevelDogfood(t *testing.T) {
	mirror := map[string]string{
		"specs/format-detector.skill.yaml":        "../../skills/format-detector.skill.yaml",
		"specs/analyzer.skill.yaml":               "../../skills/analyzer.skill.yaml",
		"specs/context-enricher.skill.yaml":       "../../skills/context-enricher.skill.yaml",
		"specs/recommendation-builder.skill.yaml": "../../skills/recommendation-builder.skill.yaml",
		"specs/doctor.agent.yaml":                 "../../agents/doctor.agent.yaml",
	}
	for embedded, topLevel := range mirror {
		emb, err := os.ReadFile(embedded)
		if err != nil {
			t.Fatalf("read embedded %s: %v", embedded, err)
		}
		top, err := os.ReadFile(topLevel)
		if err != nil {
			t.Fatalf("read top-level %s: %v (the dogfood set must mirror the embedded doctor specs)", topLevel, err)
		}
		if !bytes.Equal(emb, top) {
			t.Errorf("%s and %s have drifted — re-sync them (the doctor pipeline must be identical in both places)", embedded, topLevel)
		}
	}
}
