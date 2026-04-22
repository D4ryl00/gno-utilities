package main

import (
	"testing"

	"github.com/remi/gno-utilities/valcontrol/pkg/valcontrol"
)

func TestShouldRestartAfterClear(t *testing.T) {
	t.Parallel()

	makeState := func(rules map[string]*valcontrol.RuleView) *valcontrol.SignerState {
		return &valcontrol.SignerState{Rules: rules}
	}

	tests := []struct {
		name  string
		state *valcontrol.SignerState
		phase string
		want  bool
	}{
		{
			name:  "single prevote drop",
			state: makeState(map[string]*valcontrol.RuleView{"prevote": {Action: "drop"}}),
			phase: "prevote",
			want:  true,
		},
		{
			name:  "single precommit delay",
			state: makeState(map[string]*valcontrol.RuleView{"precommit": {Action: "delay", Delay: "5s"}}),
			phase: "precommit",
			want:  false,
		},
		{
			name:  "clear all with dropped vote",
			state: makeState(map[string]*valcontrol.RuleView{"precommit": {Action: "drop"}}),
			want:  true,
		},
		{
			name:  "clear all with proposal drop only",
			state: makeState(map[string]*valcontrol.RuleView{"proposal": {Action: "drop"}}),
			want:  false,
		},
		{
			name:  "missing state",
			state: nil,
			phase: "prevote",
			want:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldRestartAfterClear(tt.state, tt.phase); got != tt.want {
				t.Fatalf("shouldRestartAfterClear(...)= %v, want %v", got, tt.want)
			}
		})
	}
}
