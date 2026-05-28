package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// OrderingIssueType represents the type of skill ordering issue.
type OrderingIssueType string

const (
	OrderDescriptionMismatch OrderingIssueType = "description-order-mismatch"
	OrderDataFlowMismatch    OrderingIssueType = "data-flow-order-mismatch"
)

// OrderingIssue represents a problem with skill ordering in an agent.
type OrderingIssue struct {
	Type     OrderingIssueType
	Agent    string
	Message  string
	Severity string // always "warning"
}

// CheckSkillOrdering checks if skills in a sequential agent are ordered correctly.
// Returns nil for non-sequential orchestration.
func CheckSkillOrdering(agent model.AgentComposition, skillMap map[string]model.SkillBehavior) []OrderingIssue {
	if agent.Orchestration != model.OrchestrationSequential {
		return nil
	}
	var issues []OrderingIssue
	issues = append(issues, checkDescriptionOrder(agent)...)
	issues = append(issues, checkDataFlowOrder(agent, skillMap)...)
	return issues
}

type skillPosition struct {
	skill    string
	index    int
	arrayPos int
}

// checkDescriptionOrder checks if the agent's description mentions skills
// in a different order than the skills[] array.
func checkDescriptionOrder(agent model.AgentComposition) []OrderingIssue {
	if agent.Description == "" {
		return nil
	}

	desc := strings.ToLower(agent.Description)

	// Find position of each skill name in the description
	var positions []skillPosition

	for i, skillName := range agent.Skills {
		variants := []string{
			strings.ToLower(skillName),
			strings.ToLower(strings.ReplaceAll(skillName, "-", " ")),
		}

		earliest := -1
		for _, variant := range variants {
			pos := strings.Index(desc, variant)
			if pos != -1 && (earliest == -1 || pos < earliest) {
				earliest = pos
			}
		}

		if earliest != -1 {
			positions = append(positions, skillPosition{
				skill:    skillName,
				index:    earliest,
				arrayPos: i,
			})
		}
	}

	// Need at least 2 skills mentioned to detect mismatch
	if len(positions) < 2 {
		return nil
	}

	// Sort by position in description
	descOrder := make([]skillPosition, len(positions))
	copy(descOrder, positions)
	sort.Slice(descOrder, func(i, j int) bool {
		return descOrder[i].index < descOrder[j].index
	})

	// Check if description order matches array order
	isOrdered := true
	for idx := 1; idx < len(descOrder); idx++ {
		if descOrder[idx].arrayPos <= descOrder[idx-1].arrayPos {
			isOrdered = false
			break
		}
	}

	if !isOrdered {
		descNames := make([]string, len(descOrder))
		for i, p := range descOrder {
			descNames[i] = p.skill
		}

		// Sort positions by array order for the "but skills array is" part
		arrayOrder := make([]skillPosition, len(positions))
		copy(arrayOrder, positions)
		sort.Slice(arrayOrder, func(i, j int) bool {
			return arrayOrder[i].arrayPos < arrayOrder[j].arrayPos
		})
		arrayNames := make([]string, len(arrayOrder))
		for i, p := range arrayOrder {
			arrayNames[i] = p.skill
		}

		return []OrderingIssue{{
			Type:     OrderDescriptionMismatch,
			Agent:    agent.Agent,
			Message:  fmt.Sprintf("Skill ordering mismatch: description suggests [%s] but skills array is [%s]", strings.Join(descNames, " → "), strings.Join(arrayNames, " → ")),
			Severity: "warning",
		}}
	}

	return nil
}

// checkDataFlowOrder checks if a skill consumes data produced by a skill
// appearing later in the array (wrong order for sequential execution).
func checkDataFlowOrder(agent model.AgentComposition, skillMap map[string]model.SkillBehavior) []OrderingIssue {
	var issues []OrderingIssue

	// Build produces map: dataItem -> (skillName, arrayIndex)
	type producer struct {
		skillName  string
		arrayIndex int
	}
	producesMap := make(map[string]producer)

	for i, skillName := range agent.Skills {
		skill, ok := skillMap[skillName]
		if !ok {
			continue
		}
		for _, item := range skill.Context.Produces {
			producesMap[item] = producer{skillName: skillName, arrayIndex: i}
		}
	}

	// For each skill, check if it consumes something produced by a later skill
	for i, skillName := range agent.Skills {
		skill, ok := skillMap[skillName]
		if !ok {
			continue
		}
		for _, item := range skill.Context.Consumes {
			prod, exists := producesMap[item]
			if exists && prod.arrayIndex > i {
				issues = append(issues, OrderingIssue{
					Type:     OrderDataFlowMismatch,
					Agent:    agent.Agent,
					Message:  fmt.Sprintf("%q consumes %q but it is produced by %q which runs later (index %d > %d)", skillName, item, prod.skillName, prod.arrayIndex, i),
					Severity: "warning",
				})
			}
		}
	}

	return issues
}
