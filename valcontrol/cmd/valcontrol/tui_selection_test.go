package main

import (
	"testing"

	"github.com/remi/gno-utilities/valcontrol/pkg/valcontrol"
)

func TestPickRuleTargets(t *testing.T) {
	t.Parallel()

	snapshots := []valcontrol.ValidatorSnapshot{
		{Validator: &valcontrol.Validator{Name: "val1"}},
		{Validator: &valcontrol.Validator{Name: "val2"}},
		{Validator: &valcontrol.Validator{Name: "val3"}},
	}

	t.Run("falls back to focused validator", func(t *testing.T) {
		t.Parallel()

		targets := pickRuleTargets(snapshots, 1, nil)
		if got, want := targetNames(targets), []string{"val2"}; !equalStrings(got, want) {
			t.Fatalf("targetNames(...) = %v, want %v", got, want)
		}
	})

	t.Run("uses explicit selections in snapshot order", func(t *testing.T) {
		t.Parallel()

		targets := pickRuleTargets(snapshots, 0, map[string]struct{}{
			"val3": {},
			"val1": {},
		})
		if got, want := targetNames(targets), []string{"val1", "val3"}; !equalStrings(got, want) {
			t.Fatalf("targetNames(...) = %v, want %v", got, want)
		}
	})

	t.Run("stale selections fall back to focus", func(t *testing.T) {
		t.Parallel()

		targets := pickRuleTargets(snapshots, 2, map[string]struct{}{
			"missing": {},
		})
		if got, want := targetNames(targets), []string{"val3"}; !equalStrings(got, want) {
			t.Fatalf("targetNames(...) = %v, want %v", got, want)
		}
	})
}

func TestShouldClearDropForTargets(t *testing.T) {
	t.Parallel()

	withRule := func(name, phase, action string) valcontrol.ValidatorSnapshot {
		return valcontrol.ValidatorSnapshot{
			Validator: &valcontrol.Validator{Name: name},
			Signer: &valcontrol.SignerState{
				Rules: map[string]*valcontrol.RuleView{
					phase: {Action: action},
				},
			},
		}
	}

	t.Run("true when all targets already drop the phase", func(t *testing.T) {
		t.Parallel()

		targets := []valcontrol.ValidatorSnapshot{
			withRule("val1", "prevote", "drop"),
			withRule("val2", "prevote", "drop"),
		}
		if !shouldClearDropForTargets(targets, "prevote") {
			t.Fatal("shouldClearDropForTargets(...) = false, want true")
		}
	})

	t.Run("false for mixed selections", func(t *testing.T) {
		t.Parallel()

		targets := []valcontrol.ValidatorSnapshot{
			withRule("val1", "prevote", "drop"),
			withRule("val2", "prevote", "delay"),
		}
		if shouldClearDropForTargets(targets, "prevote") {
			t.Fatal("shouldClearDropForTargets(...) = true, want false")
		}
	})
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
