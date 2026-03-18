package api

import "testing"

func TestIsAIInvestigationEnabled_DefaultFalse(t *testing.T) {
	SetAIInvestigationEnabled(nil)
	t.Cleanup(func() { SetAIInvestigationEnabled(nil) })

	if isAIInvestigationEnabled() {
		t.Fatal("expected false when no hook is registered")
	}
}

func TestIsAIInvestigationEnabled_TrueWhenSet(t *testing.T) {
	SetAIInvestigationEnabled(nil)
	t.Cleanup(func() { SetAIInvestigationEnabled(nil) })

	SetAIInvestigationEnabled(func() bool { return true })
	if !isAIInvestigationEnabled() {
		t.Fatal("expected true when hook returns true")
	}
}

func TestIsAIInvestigationEnabled_FalseWhenHookReturnsFalse(t *testing.T) {
	SetAIInvestigationEnabled(nil)
	t.Cleanup(func() { SetAIInvestigationEnabled(nil) })

	SetAIInvestigationEnabled(func() bool { return false })
	if isAIInvestigationEnabled() {
		t.Fatal("expected false when hook returns false")
	}
}
