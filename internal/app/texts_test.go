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

func TestQA_PORT_02_TheConsentNamesTheDistroAndOnlyTheHelper(t *testing.T) {
	got := consentMessage("Ubuntu")

	if !strings.Contains(got, "Ubuntu") {
		t.Errorf("consent = %q, want it to name the distro", got)
	}
	for _, unwanted := range []string{"command", "login shell"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("consent = %q, want no mention of %q", got, unwanted)
		}
	}
}

func TestFR_UX_11_TextWithNoArgsReturnsThePlainEntry(t *testing.T) {
	if got := text(appName, nil); got != "nebegit" {
		t.Errorf("text(appName, nil) = %q, want %q", got, "nebegit")
	}
}
