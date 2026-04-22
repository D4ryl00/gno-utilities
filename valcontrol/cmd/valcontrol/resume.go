package main

import (
	"fmt"

	"github.com/remi/gno-utilities/valcontrol/pkg/valcontrol"
)

func shouldRestartAfterClear(state *valcontrol.SignerState, phase string) bool {
	if state == nil {
		return false
	}
	if phase != "" {
		return isDroppedVoteRule(state.Rules[phase], phase)
	}
	for rulePhase, rule := range state.Rules {
		if isDroppedVoteRule(rule, rulePhase) {
			return true
		}
	}
	return false
}

func isDroppedVoteRule(rule *valcontrol.RuleView, phase string) bool {
	if rule == nil || rule.Action != "drop" {
		return false
	}
	return phase == "prevote" || phase == "precommit"
}

func restartValidator(inv *valcontrol.Inventory, scenarioLib, name string) error {
	libPath, err := resolveScenarioLib(scenarioLib)
	if err != nil {
		return err
	}

	script := buildScenarioFuncScript(libPath, inv, fmt.Sprintf("stop_validator %q\nstart_validator %q", name, name))
	if out, err := runBashScript(script); err != nil {
		return fmt.Errorf("restart %s: %w\n%s", name, err, out)
	}

	return nil
}
