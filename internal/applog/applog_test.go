package applog

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestFR_UX_02_OpenCreatesTheLogUnderXDGStateHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	logger, path, closer, _, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if want := filepath.Join(tmp, "nebegit", "nebegit.log"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	if logger == nil {
		t.Fatal("logger is nil")
	}
}

func TestFR_UX_02_OpenRotatesAnOversizedLog(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	dir := filepath.Join(tmp, "nebegit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "nebegit.log")
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

func TestFR_UX_19_OpenStartsAtInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	_, _, closer, level, err := Open(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if level.Level() != slog.LevelInfo {
		t.Errorf("level = %v, want Info", level.Level())
	}
}

func TestFR_UX_19_OpenStartsAtDebugWhenNebegitLogIsDebug(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
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

func TestFR_UX_19_SetLevelDebugModeTrueSetsDebug(t *testing.T) {
	level := &slog.LevelVar{}

	SetLevel(level, true)

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug", level.Level())
	}
}

func TestFR_UX_19_SetLevelDebugModeFalseSetsInfo(t *testing.T) {
	level := &slog.LevelVar{}
	level.Set(slog.LevelDebug)

	SetLevel(level, false)

	if level.Level() != slog.LevelInfo {
		t.Errorf("level = %v, want Info", level.Level())
	}
}

func TestFR_UX_19_SetLevelNebegitLogDebugForcesDebugEvenWhenDebugModeFalse(t *testing.T) {
	t.Setenv("NEBEGIT_LOG", "debug")
	level := &slog.LevelVar{}

	SetLevel(level, false)

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug (forced by NEBEGIT_LOG)", level.Level())
	}
}
