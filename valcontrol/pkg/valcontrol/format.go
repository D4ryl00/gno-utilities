package valcontrol

import (
	"fmt"
	"sort"
	"strings"
)

func FormatRules(state *SignerState) string {
	if state == nil || len(state.Rules) == 0 {
		return "-"
	}

	phases := make([]string, 0, len(state.Rules))
	for phase := range state.Rules {
		phases = append(phases, phase)
	}
	sort.Strings(phases)

	parts := make([]string, 0, len(phases))
	for _, phase := range phases {
		rule := state.Rules[phase]
		if rule == nil {
			continue
		}
		part := phase + ":" + rule.Action
		if rule.Delay != "" {
			part += "(" + rule.Delay + ")"
		}
		if rule.Height != nil || rule.Round != nil {
			part += "@"
			if rule.Height != nil {
				part += fmt.Sprintf("h%d", *rule.Height)
			}
			if rule.Round != nil {
				part += fmt.Sprintf("r%d", *rule.Round)
			}
		}
		parts = append(parts, part)
	}

	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, ",")
}
