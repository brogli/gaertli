// Requirements: FR-UX-11, QA-PORT-02

package app

import (
	"strings"
	"testing"
)

func TestTextFillsNamedPlaceholders(t *testing.T) {
	got := text(worktreeTitle, map[string]string{"name": "project", "branch": "main"})
	if want := "project [main]"; got != want {
		t.Errorf("text(worktreeTitle, ...) = %q, want %q", got, want)
	}
}

func TestTheConsentNamesTheDistroAndOnlyTheHelper(t *testing.T) {
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

func TestTextWithNoArgsReturnsThePlainEntry(t *testing.T) {
	if got := text(appName, nil); got != "Gärtli" {
		t.Errorf("text(appName, nil) = %q, want %q", got, "Gärtli")
	}
}
