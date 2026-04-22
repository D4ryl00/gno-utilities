package valcontrol

import "testing"

func TestFindValidator(t *testing.T) {
	inv := &Inventory{Validators: []Validator{{Name: "val1"}, {Name: "val2"}}}
	val, err := inv.FindValidator("val2")
	if err != nil {
		t.Fatalf("FindValidator() error = %v", err)
	}
	if val.Name != "val2" {
		t.Fatalf("unexpected validator: %+v", val)
	}
}

func TestFormatRules(t *testing.T) {
	height := int64(12)
	state := &SignerState{Rules: map[string]*RuleView{
		"precommit": {Action: "drop", Height: &height},
	}}
	got := FormatRules(state)
	if got == "-" || got == "" {
		t.Fatalf("unexpected rules summary: %q", got)
	}
}
