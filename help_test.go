package main

import "testing"

// allHelpTerms flattens the three help sections for whole-menu checks.
func allHelpTerms() []helpTerm {
	out := make([]helpTerm, 0, len(helpVerbs)+len(helpModifiers)+len(helpExamples))
	out = append(out, helpVerbs...)
	out = append(out, helpModifiers...)
	out = append(out, helpExamples...)
	return out
}

func TestHelpListsAllBindings(t *testing.T) {
	required := []string{
		"W", "S", "A", "D", "Q", "E",
		"j", "g", "h", "n", "p", "u", "=",
		"[", "]",
		"j [", "j ]",
		"Space / Esc", "?",
	}
	have := make(map[string]bool)
	for _, e := range allHelpTerms() {
		have[e.keys] = true
	}
	for _, r := range required {
		if !have[r] {
			t.Errorf("help is missing binding %q", r)
		}
	}
}

func TestHelpNoDuplicateBindings(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range allHelpTerms() {
		if seen[e.keys] {
			t.Errorf("duplicate help binding %q", e.keys)
		}
		seen[e.keys] = true
	}
}

func TestHelpEveryTermHasAPhrase(t *testing.T) {
	for _, e := range allHelpTerms() {
		if e.phrase == "" {
			t.Errorf("binding %q has no description", e.keys)
		}
	}
}

func TestHelpPlannedFlags(t *testing.T) {
	planned := map[string]bool{
		"g": true, "h": true, "n": true, "p": true, "u": true, "=": true,
	}
	implemented := map[string]bool{
		"W": true, "S": true, "A": true, "D": true, "Q": true, "E": true,
		"j": true, "[": true, "]": true,
		"j [": true, "j ]": true,
		"Space / Esc": true, "?": true,
	}
	for _, e := range allHelpTerms() {
		if planned[e.keys] && !e.planned {
			t.Errorf("%q is not yet implemented but isn't marked planned", e.keys)
		}
		if implemented[e.keys] && e.planned {
			t.Errorf("%q is bound in code but is marked planned", e.keys)
		}
	}
}

func TestHelpVerbsLivedInTheVerbColumn(t *testing.T) {
	for _, e := range helpVerbs {
		if e.keys == "p" && e.phrase != "Pull item from backpack" {
			t.Fatalf("p phrase = %q, want %q", e.phrase, "Pull item from backpack")
		}
	}
}

func TestHelpPanelFitsViewport(t *testing.T) {
	if h := helpPanelHeight(); h >= screenHeight {
		t.Fatalf("help panel %dpx must fit inside the %dpx viewport", h, screenHeight)
	}
	if len(helpVerbs) == 0 || len(helpExamples) == 0 {
		t.Fatal("help sections are empty")
	}
}
