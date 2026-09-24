package app

import "testing"

func TestFR_UX_11_TextFillsNamedPlaceholders(t *testing.T) {
	got := text(worktreeTitle, map[string]string{"name": "project", "branch": "main"})
	if want := "project [main]"; got != want {
		t.Errorf("text(worktreeTitle, ...) = %q, want %q", got, want)
	}
}

func TestFR_UX_11_TextWithNoArgsReturnsThePlainEntry(t *testing.T) {
	if got := text(appName, nil); got != "nebegit" {
		t.Errorf("text(appName, nil) = %q, want %q", got, "nebegit")
	}
}
