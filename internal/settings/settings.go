// Package settings persists gaertli's own configuration as JSON in the OS
// user config directory. No Wails/cgo import.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/brogli/gaertli/internal/core"
)

// maxRecent bounds RecentRepos.
const maxRecent = 10

// Settings is the whole persisted file: the shell's own record of recent
// repos, open windows and the diff window's size, and the user's
// Preferences.
type Settings struct {
	RecentRepos    []RecentRepo `json:"recentRepos"`              // most-recent-first, capped at maxRecent
	OpenWorktrees  []string     `json:"openWorktrees"`            // windows open at last quit
	DiffWindowSize *WindowSize  `json:"diffWindowSize,omitempty"` // nil until a diff window has closed once
	Preferences
}

// WindowSize is a window's width and height, in the units Wails' own
// Size() reports and WebviewWindowOptions takes.
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Preferences are the settings the user edits; the frontend reads and saves
// them through SettingsService.
type Preferences struct {
	// EditorProfile is the one editor every environment opens files with;
	// a WSL repo runs its command inside the distro.
	EditorProfile core.EditorProfile `json:"editorProfile"`

	ProtectedBranches        []string      `json:"protectedBranches"`
	DefaultPullMode          core.PullMode `json:"defaultPullMode"`
	DiffMode                 DiffMode      `json:"diffMode"`
	AutoFetchIntervalMinutes int           `json:"autoFetchIntervalMinutes"` // 0 = disabled
	Theme                    Theme         `json:"theme"`
	IgnoreWhitespace         bool          `json:"ignoreWhitespace"` // the diff view's whitespace toggle
	DebugMode                bool          `json:"debugMode"`        // each window shows the debug strip

	// WorktreeFolders is the base folder for new worktrees, keyed by
	// environment: "" for local, else a WSL distro name. An absent entry
	// means the folder holding the repo's main worktree.
	WorktreeFolders map[string]string `json:"worktreeFolders,omitempty"`
}

// DiffMode is how the diff view lays out a file's two sides.
type DiffMode string

const (
	UnifiedDiff    DiffMode = "unified"
	SideBySideDiff DiffMode = "side-by-side"
)

// Theme is the UI's colour scheme.
type Theme string

const (
	SystemTheme          Theme = "" // follows the OS preference
	LightTheme           Theme = "light"
	DarkTheme            Theme = "dark"
	GruvboxDarkTheme     Theme = "gruvbox-dark"
	CatppuccinMochaTheme Theme = "catppuccin-mocha"
	NordTheme            Theme = "nord"
	DraculaTheme         Theme = "dracula"
)

// themeValues are Theme's allowed values.
var themeValues = map[Theme]bool{
	SystemTheme: true, LightTheme: true, DarkTheme: true,
	GruvboxDarkTheme: true, CatppuccinMochaTheme: true, NordTheme: true, DraculaTheme: true,
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

// Default is a fresh install's settings: no recents, nothing open, main and
// master protected, git's own pull default (--no-rebase), unified diffs,
// and no auto-fetch.
func Default() Settings {
	return Settings{ProtectedBranches: []string{"main", "master"}, DefaultPullMode: core.PullMerge, DiffMode: UnifiedDiff}
}

// Path is <os.UserConfigDir()>/gaertli/settings.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gaertli", "settings.json"), nil
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
	if s.EditorProfile == (core.EditorProfile{}) {
		s.EditorProfile = legacyEditorProfile(data)
	}
	if !themeValues[s.Theme] {
		s.Theme = SystemTheme
	}
	return s, nil
}

// legacyEditorProfile picks the single profile out of an older file's
// per-environment "editorProfiles": the first filled one by environment
// name, so the local ("") entry wins.
func legacyEditorProfile(data []byte) core.EditorProfile {
	var old struct {
		EditorProfiles map[string]core.EditorProfile `json:"editorProfiles"`
	}
	if json.Unmarshal(data, &old) != nil {
		return core.EditorProfile{}
	}
	for _, env := range slices.Sorted(maps.Keys(old.EditorProfiles)) {
		if p := old.EditorProfiles[env]; p != (core.EditorProfile{}) {
			return p
		}
	}
	return core.EditorProfile{}
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

// RemoveRecent drops every entry that sameRepo(repo), keeping the others in
// order.
func (s *Settings) RemoveRecent(repo RecentRepo) {
	entries := make([]RecentRepo, 0, len(s.RecentRepos))
	for _, r := range s.RecentRepos {
		if !r.sameRepo(repo) {
			entries = append(entries, r)
		}
	}
	s.RecentRepos = entries
}
