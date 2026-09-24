package applog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFR_UX_02_OpenCreatesTheLogUnderXDGStateHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	logger, path, closer, err := Open()
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

	_, _, closer, err := Open()
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
