package app

import (
	"strings"
	"testing"
)

func TestFR_UX_11_TextFillsNamedPlaceholders(t *testing.T) {
	got := text(worktreeTitle, map[string]string{"name": "project", "branch": "main"})
	if want := "project [main]"; got != want {
		t.Errorf("text(worktreeTitle, ...) = %q, want %q", got, want)
	}
}

func TestQA_PORT_02_TheConsentNamesTheDistroAndTheNextLoginShellCaveat(t *testing.T) {
	got := consentMessage("Ubuntu")

	for _, want := range []string{"Ubuntu", "next login shell"} {
		if !strings.Contains(got, want) {
			t.Errorf("consent = %q, want it to name %q", got, want)
		}
	}
}

func TestFR_UX_11_TextWithNoArgsReturnsThePlainEntry(t *testing.T) {
	if got := text(appName, nil); got != "nebegit" {
		t.Errorf("text(appName, nil) = %q, want %q", got, "nebegit")
	}
}
