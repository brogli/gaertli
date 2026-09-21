// Package settings persists nebegit's own configuration as JSON in the OS
// user config directory (TR-13). No Wails/cgo import (TR-07).
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// maxRecent bounds RecentRepos.
const maxRecent = 10

// Settings is the whole persisted file.
type Settings struct {
	RecentRepos              []RecentRepo `json:"recentRepos"`              // FR-REPO-02, most-recent-first, capped at maxRecent
	OpenWorktrees            []string     `json:"openWorktrees"`            // FR-REPO-02: windows open at last quit
	Editor                   string       `json:"editor"`                   // FR-UX-04; "" = built-in default
	ProtectedBranches        []string     `json:"protectedBranches"`        // FR-UX-04
	DefaultPullMode          string       `json:"defaultPullMode"`          // FR-UX-04: "merge" | "rebase"
	DiffMode                 string       `json:"diffMode"`                 // FR-UX-04: "unified" | "side-by-side"
	AutoFetchIntervalMinutes int          `json:"autoFetchIntervalMinutes"` // FR-UX-04; 0 = disabled
}

// RecentRepo is one entry of Settings.RecentRepos.
type RecentRepo struct {
	Path       string `json:"path"`
	LastOpened string `json:"lastOpened"` // RFC3339
}

// Default is a fresh install's settings: no recents, nothing open, git's own
// pull default (--no-rebase), unified diffs, and no unasked-for network
// traffic (QA-SEC-01).
func Default() Settings {
	return Settings{DefaultPullMode: "merge", DiffMode: "unified"}
}

// Path is <os.UserConfigDir()>/nebegit/settings.json (TR-13).
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
// earlier entry for the same path, capped at maxRecent.
func (s *Settings) AddRecent(path string, now time.Time) {
	entries := []RecentRepo{{Path: path, LastOpened: now.Format(time.RFC3339)}}
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

// SetOpenWorktrees replaces the FR-REPO-02 restore snapshot.
func (s *Settings) SetOpenWorktrees(paths []string) {
	s.OpenWorktrees = paths
}
