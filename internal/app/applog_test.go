// Requirements: FR-UX-02

package app

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

// dispatchLogged runs a name event carrying record for wt through a
// Registry whose log writes to a fresh buffer at level, and returns what
// was logged.
func dispatchLogged(t *testing.T, level slog.Level, name string, wt core.WorktreeID, record any) string {
	t.Helper()
	r, _ := newTestRegistry()
	var logged strings.Builder
	r.log = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: level}))

	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	r.Dispatch(core.Event{Name: name, Worktree: wt, Payload: payload})
	return logged.String()
}

func TestASuccessfulCommandAtInfoLevelLogsNothing(t *testing.T) {
	out := dispatchLogged(t, slog.LevelInfo, "command", "wt1", core.CommandRecord{
		Command: "git status", Dir: "/repo", DurationMS: 12, ExitCode: 0,
	})

	if out != "" {
		t.Errorf("log = %q, want no record for a successful command at Info level", out)
	}
}

func TestASuccessfulCommandAtDebugLevelLogsOneDebugRecord(t *testing.T) {
	out := dispatchLogged(t, slog.LevelDebug, "command", "wt1", core.CommandRecord{
		Command: "git status", Dir: "/repo", DurationMS: 12, ExitCode: 0,
	})

	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("log = %q, want level=DEBUG", out)
	}
	for _, want := range []string{`cmd=`, `"git status"`, `dir=/repo`, `exit=0`, `ms=12`, `worktree=wt1`} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want it to contain %q", out, want)
		}
	}
}

func TestABackgroundCommandAtDebugLevelLogsOneDebugRecord(t *testing.T) {
	out := dispatchLogged(t, slog.LevelDebug, "command", "wt1", core.CommandRecord{
		Command: "git status", Background: true, ExitCode: 0,
	})

	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("log = %q, want level=DEBUG", out)
	}
}

func TestAFailedCommandLogsWarnEvenAtInfoLevel(t *testing.T) {
	out := dispatchLogged(t, slog.LevelInfo, "command", "wt1", core.CommandRecord{
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

func TestAnEditorStartIsLoggedAtDebugWithItsArgv(t *testing.T) {
	out := dispatchLogged(t, slog.LevelDebug, "editor", "wt1", core.EditorRecord{Argv: []string{"alpha", "/repo"}})

	if strings.Contains(out, "level=INFO") || strings.Contains(out, "level=WARN") {
		t.Errorf("log = %q, want no Info or Warn record for a started editor", out)
	}
	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("log = %q, want level=DEBUG", out)
	}
	for _, want := range []string{`argv=`, `alpha`, `/repo`, `worktree=wt1`} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want it to contain %q", out, want)
		}
	}
}

func TestAFailedEditorStartIsLoggedAtWarnWithItsArgvAndError(t *testing.T) {
	out := dispatchLogged(t, slog.LevelDebug, "editor", "wt1", core.EditorRecord{
		Argv: []string{"myed", "/repo"}, Error: `"myed" is not on PATH`,
	})

	if !strings.Contains(out, "level=WARN") {
		t.Errorf("log = %q, want level=WARN", out)
	}
	for _, want := range []string{`argv=`, `myed`, `err=`, "not on PATH"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want it to contain %q", out, want)
		}
	}
}

func TestCommandAndEditorEventsAreNeverEmittedToAWindow(t *testing.T) {
	records := map[string]any{
		"command": core.CommandRecord{Command: "git status"},
		"editor":  core.EditorRecord{Argv: []string{"alpha"}},
	}
	for name, record := range records {
		t.Run(name, func(t *testing.T) {
			r, opener := newTestRegistry()
			r.OpenOrFocus("/a", "/a", "")

			payload, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			r.Dispatch(core.Event{Name: name, Worktree: "/a", Payload: payload})

			if n := len(opener.windows[0].emitted); n != 0 {
				t.Errorf("emitted %d events, want 0 (logged instead)", n)
			}
		})
	}
}
