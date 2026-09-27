// Package applog opens nebegit's own log file: one process, one text
// handler, rotated by size at open.
package applog

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

// maxSize is how large the log file may grow before Open rotates it.
const maxSize = 5 * 1024 * 1024

// Open opens (creating if needed) nebegit's log file at its platform
// location, rotating an oversized one first, and returns a logger writing
// to it and the LevelVar controlling its level, set by SetLevel from
// debugMode. NEBEGIT_LOG=debug forces Debug for the whole run regardless of
// later SetLevel calls, for problems before Settings load.
//
// On failure the returned logger still works, writing to stderr instead;
// path is "" and closer is nil.
func Open(debugMode bool) (logger *slog.Logger, path string, closer io.Closer, level *slog.LevelVar, err error) {
	level = &slog.LevelVar{}
	SetLevel(level, debugMode)
	opts := &slog.HandlerOptions{Level: level}

	f, path, err := openFile()
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, opts)), "", nil, level, err
	}
	return slog.New(slog.NewTextHandler(f, opts)), path, f, level, nil
}

// SetLevel sets level to Debug when debugMode is true or NEBEGIT_LOG=debug
// is set, else Info.
func SetLevel(level *slog.LevelVar, debugMode bool) {
	if debugMode || os.Getenv("NEBEGIT_LOG") == "debug" {
		level.Set(slog.LevelDebug)
		return
	}
	level.Set(slog.LevelInfo)
}

// openFile opens the log file for appending, rotating an oversized one
// first.
func openFile() (*os.File, string, error) {
	path, err := logPath()
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, "", err
	}
	rotate(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	return f, path, err
}

// rotate renames an oversized path to path+".1", replacing an older one. A
// failed rename leaves Open appending to the existing file.
func rotate(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxSize {
		return
	}
	_ = os.Rename(path, path+".1")
}

// logPath is nebegit's log file for the current platform.
func logPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// os.UserCacheDir is %LOCALAPPDATA% on Windows.
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "nebegit", "logs", "nebegit.log"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Logs", "nebegit", "nebegit.log"), nil
	default:
		dir := os.Getenv("XDG_STATE_HOME")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(dir, "nebegit", "nebegit.log"), nil
	}
}
