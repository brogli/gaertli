// Package settings persists nebegit's own configuration as JSON in the OS
// user config directory. No Wails/cgo import.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// maxRecent bounds RecentRepos.
const maxRecent = 10

// Settings is the whole persisted file.
type Settings struct {
	RecentRepos   []RecentRepo `json:"recentRepos"`   // most-recent-first, capped at maxRecent
	OpenWorktrees []string     `json:"openWorktrees"` // windows open at last quit

	// EditorProfiles is keyed by environment: "" for local, else a WSL
	// distro name.
	EditorProfiles map[string]core.EditorProfile `json:"editorProfiles,omitempty"`

	ProtectedBranches        []string `json:"protectedBranches"`
	DefaultPullMode          string   `json:"defaultPullMode"`          // "merge" | "rebase"
	DiffMode                 string   `json:"diffMode"`                 // "unified" | "side-by-side"
	AutoFetchIntervalMinutes int      `json:"autoFetchIntervalMinutes"` // 0 = disabled
}

// RecentRepo is one entry of Settings.RecentRepos.
type RecentRepo struct {
	Path       string `json:"path"`
	LastOpened string `json:"lastOpened"`       // RFC3339
	Distro     string `json:"distro,omitempty"` // "" = local; else the WSL distro name
}

// Default is a fresh install's settings: no recents, nothing open, git's own
// pull default (--no-rebase), unified diffs, and no auto-fetch.
func Default() Settings {
	return Settings{DefaultPullMode: "merge", DiffMode: "unified"}
}

// Path is <os.UserConfigDir()>/nebegit/settings.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nebegit", "settings.json"), nil
}

// Load reads path. A missing file returns Default(), nil (first run).
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save writes s to path atomically: marshal indented JSON to a temp file in
// the same directory, then rename over path.
func Save(path string, s Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AddRecent inserts path at the front with LastOpened = now, removing any
// earlier entry for the same path, capped at maxRecent. distro is "" for a
// local repo, else the WSL distro it was opened from.
func (s *Settings) AddRecent(path string, now time.Time, distro string) {
	entries := []RecentRepo{{Path: path, LastOpened: now.Format(time.RFC3339), Distro: distro}}
	for _, r := range s.RecentRepos {
		if r.Path != path {
			entries = append(entries, r)
		}
	}
	if len(entries) > maxRecent {
		entries = entries[:maxRecent]
	}
	s.RecentRepos = entries
}

// SetOpenWorktrees replaces the restore-at-launch snapshot.
func (s *Settings) SetOpenWorktrees(paths []string) {
	s.OpenWorktrees = paths
}
