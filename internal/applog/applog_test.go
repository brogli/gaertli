// Requirements: FR-UX-02, FR-UX-19

package applog

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// platformStateDir points the current platform's state-directory variable
// (Linux XDG_STATE_HOME, Windows LocalAppData, macOS HOME) at a fresh temp
// dir and returns the log path Open should use under it.
func platformStateDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LocalAppData", tmp)
		return filepath.Join(tmp, "nebegit", "logs", "nebegit.log")
	case "darwin":
		t.Setenv("HOME", tmp)
		return filepath.Join(tmp, "Library", "Logs", "nebegit", "nebegit.log")
	default:
		t.Setenv("XDG_STATE_HOME", tmp)
		return filepath.Join(tmp, "nebegit", "nebegit.log")
	}
}

func TestOpenCreatesTheLogUnderThePlatformsStateDir(t *testing.T) {
	want := platformStateDir(t)

	logger, path, closer, _, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no owner-only mode bits; the folder is per-user by its access list")
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}
	})
	if logger == nil {
		t.Fatal("logger is nil")
	}
}

func TestOpenRotatesAnOversizedLog(t *testing.T) {
	path := platformStateDir(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	old := bytes.Repeat([]byte("x"), maxSize+1)
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, closer, _, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	rotated, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rotated, old) {
		t.Error("nebegit.log.1 does not hold the old content")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("new log size = %d, want 0", info.Size())
	}
}

func TestOpenStartsAtInfo(t *testing.T) {
	platformStateDir(t)

	_, _, closer, level, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if level.Level() != slog.LevelInfo {
		t.Errorf("level = %v, want Info", level.Level())
	}
}

func TestOpenStartsAtDebugWhenNebegitLogIsDebug(t *testing.T) {
	platformStateDir(t)
	t.Setenv("NEBEGIT_LOG", "debug")

	_, _, closer, level, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug", level.Level())
	}
}

func TestSetLevelDebugModeTrueSetsDebug(t *testing.T) {
	level := &slog.LevelVar{}

	SetLevel(level, true)

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug", level.Level())
	}
}

func TestSetLevelDebugModeFalseSetsInfo(t *testing.T) {
	level := &slog.LevelVar{}
	level.Set(slog.LevelDebug)

	SetLevel(level, false)

	if level.Level() != slog.LevelInfo {
		t.Errorf("level = %v, want Info", level.Level())
	}
}

func TestSetLevelNebegitLogDebugForcesDebugEvenWhenDebugModeFalse(t *testing.T) {
	t.Setenv("NEBEGIT_LOG", "debug")
	level := &slog.LevelVar{}

	SetLevel(level, false)

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug (forced by NEBEGIT_LOG)", level.Level())
	}
}
