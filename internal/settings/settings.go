// Package settings persists nebegit's own configuration as JSON in the OS
// user config directory. No Wails/cgo import.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
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
	Theme                    string   `json:"theme"`                    // "" = follow the OS, "light", "dark"
	IgnoreWhitespace         bool     `json:"ignoreWhitespace"`         // the diff view's whitespace toggle
}

// RecentRepo is one entry of Settings.RecentRepos.
type RecentRepo struct {
	Path       string `json:"path"`
	LastOpened string `json:"lastOpened"`        // RFC3339
	Distro     string `json:"distro,omitempty"`  // "" = local; else the WSL distro name
	RepoKey    string `json:"repoKey,omitempty"` // common git dir, shared by every worktree of the repo
}

// sameRepo reports whether r and other name one repo in one environment:
// the same path, or worktrees sharing a common git dir.
func (r RecentRepo) sameRepo(other RecentRepo) bool {
	if r.Distro != other.Distro {
		return false
	}
	return r.Path == other.Path || (r.RepoKey != "" && r.RepoKey == other.RepoKey)
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
	if errors.Is(err, fs.ErrNotExist) {
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
// the same directory, then rename over path. The directory and the file are
// readable by their owner only, also when an older install made them wider.
func Save(path string, s Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AddRecent moves repo to the front of RecentRepos, stamped now, capped at
// maxRecent. A worktree of a repo already listed replaces that entry, so
// each repo is listed once, by the worktree opened last: one that surely
// still exists, unlike an earlier one that may have been removed since.
func (s *Settings) AddRecent(repo RecentRepo, now time.Time) {
	repo.LastOpened = now.Format(time.RFC3339)
	entries := []RecentRepo{repo}
	for _, r := range s.RecentRepos {
		if !r.sameRepo(repo) {
			entries = append(entries, r)
		}
	}
	s.RecentRepos = entries[:min(len(entries), maxRecent)]
}
