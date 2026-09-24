package app

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

// dispatchCommand runs a "command" event for wt through a Registry whose
// log writes to a fresh buffer, and returns what was logged.
func dispatchCommand(t *testing.T, wt core.WorktreeID, entry core.CommandRecord) string {
	t.Helper()
	r, _ := newTestRegistry()
	var logged strings.Builder
	r.log = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	payload, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	r.Dispatch(core.Event{Name: "command", Worktree: wt, Payload: payload})
	return logged.String()
}

func TestFR_UX_02_AUserStartedCommandIsLoggedAtInfo(t *testing.T) {
	out := dispatchCommand(t, "wt1", core.CommandRecord{
		Command: "git status", Dir: "/repo", DurationMS: 12, ExitCode: 0,
	})

	if !strings.Contains(out, "level=INFO") {
		t.Errorf("log = %q, want level=INFO", out)
	}
	for _, want := range []string{`cmd=`, `"git status"`, `dir=/repo`, `exit=0`, `ms=12`, `worktree=wt1`} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want it to contain %q", out, want)
		}
	}
}

func TestFR_UX_02_ABackgroundCommandIsLoggedAtDebugNotInfo(t *testing.T) {
	out := dispatchCommand(t, "wt1", core.CommandRecord{
		Command: "git status", Background: true, ExitCode: 0,
	})

	if strings.Contains(out, "level=INFO") {
		t.Errorf("log = %q, want no Info record for a background command", out)
	}
	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("log = %q, want a Debug record", out)
	}
}

func TestFR_UX_02_AFailedUserStartedCommandIsLoggedAtWarnWithMaskedStderr(t *testing.T) {
	out := dispatchCommand(t, "wt1", core.CommandRecord{
		Command:   "git fetch",
		ExitCode:  128,
		ErrorCode: "GIT_FAILED",
		Stderr:    "fatal: could not read from 'https://u:secret@h/r'",
	})

	if !strings.Contains(out, "level=WARN") {
		t.Errorf("log = %q, want level=WARN", out)
	}
	if !strings.Contains(out, "code=GIT_FAILED") {
		t.Errorf("log = %q, want code=GIT_FAILED", out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("log = %q, want no credential", out)
	}
	if !strings.Contains(out, "https://***@h/r") {
		t.Errorf("log = %q, want the masked URL", out)
	}
}

func TestFR_UX_02_ACommandEventIsNeverEmittedToAWindow(t *testing.T) {
	r, opener := newTestRegistry()
	r.OpenOrFocus("/a", "/a", "")

	payload, err := json.Marshal(core.CommandRecord{Command: "git status"})
	if err != nil {
		t.Fatal(err)
	}
	r.Dispatch(core.Event{Name: "command", Worktree: "/a", Payload: payload})

	if n := len(opener.windows[0].emitted); n != 0 {
		t.Errorf("emitted %d events, want 0 (logged instead)", n)
	}
}
