// Requirements: FR-ED-11, FR-REPO-02, FR-REPO-07, FR-REPO-08, FR-SYNC-06, FR-UX-04, FR-UX-06, FR-WT-16, QA-SEC-02

package settings_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

func TestPathIsUnderTheOSUserConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := settings.Path()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join("nebegit", "settings.json")
	if !strings.HasSuffix(got, want) {
		t.Errorf("Path() = %q, want a suffix of %q", got, want)
	}
}

// saveAndLoad saves s to a fresh settings file and loads it back.
func saveAndLoad(t *testing.T, s settings.Settings) settings.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(path, s); err != nil {
		t.Fatal(err)
	}
	return load(t, path)
}

// loadJSON loads a settings file that holds raw.
func loadJSON(t *testing.T, raw string) settings.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return load(t, path)
}

func load(t *testing.T, path string) settings.Settings {
	t.Helper()
	s, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSaveThenLoadRoundTripsEveryField(t *testing.T) {
	want := settings.Settings{
		RecentRepos:              []settings.RecentRepo{{Path: "/a", LastOpened: "2026-09-21T00:00:00Z"}},
		OpenWorktrees:            []string{"/a", "/b"},
		DiffWindowSize:           &settings.WindowSize{Width: 900, Height: 600},
		EditorProfile:            core.EditorProfile{Name: "Alpha", FolderCmd: "alpha {path}"},
		ProtectedBranches:        []string{"main"},
		DefaultPullMode:          core.PullRebase,
		DiffMode:                 settings.SideBySideDiff,
		AutoFetchIntervalMinutes: 5,
		Theme:                    settings.DarkTheme,
		IgnoreWhitespace:         true,
		DebugMode:                true,
		WorktreeFolders:          map[string]string{"": "/wt", "Ubuntu": "/home/u/wt"},
	}

	if got := saveAndLoad(t, want); !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

// An install from before the modes were set has a 0755 directory and a
// 0644 file; its next save tightens both.
func TestASavedStoreHasAFileAndDirectoryOnlyItsOwnerCanRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	for _, existing := range []bool{false, true} {
		dir := filepath.Join(t.TempDir(), "nebegit")
		path := filepath.Join(dir, "settings.json")
		if existing {
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		store, err := settings.Open(path)
		if err != nil {
			t.Fatal(err)
		}

		if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
			t.Fatal(err)
		}

		for p, want := range map[string]fs.FileMode{path: 0o600, dir: 0o700} {
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != want {
				t.Errorf("existing=%v: %s mode = %v, want %v", existing, p, got, want)
			}
		}
	}
}

func TestLoadOnMissingFileReturnsDefault(t *testing.T) {
	got := load(t, filepath.Join(t.TempDir(), "does-not-exist.json"))

	if !reflect.DeepEqual(got, settings.Default()) {
		t.Errorf("Load() = %+v, want Default() %+v", got, settings.Default())
	}
}

func TestAFreshInstallProtectsMainAndMaster(t *testing.T) {
	got := load(t, filepath.Join(t.TempDir(), "does-not-exist.json"))

	if want := []string{"main", "master"}; !slices.Equal(got.ProtectedBranches, want) {
		t.Errorf("ProtectedBranches = %q, want %q", got.ProtectedBranches, want)
	}
}

func TestAnExistingFileKeepsTheProtectedBranchesItStores(t *testing.T) {
	got := loadJSON(t, `{"protectedBranches": null}`)

	if len(got.ProtectedBranches) != 0 {
		t.Errorf("ProtectedBranches = %q, want none, as stored", got.ProtectedBranches)
	}
}

func TestThemeRoundTrips(t *testing.T) {
	got := saveAndLoad(t, settings.Settings{Theme: settings.DarkTheme})

	if got.Theme != settings.DarkTheme {
		t.Errorf("Theme = %q, want %q", got.Theme, "dark")
	}
}

func TestLoadOnMissingThemeKeyDefaultsToFollowTheOS(t *testing.T) {
	got := loadJSON(t, `{}`)

	if got.Theme != "" {
		t.Errorf("Theme = %q, want empty (follow the OS)", got.Theme)
	}
}

func TestLoadAcceptsTheFourDarkThemesAndFallsBackToSystemOnAnUnknownValue(t *testing.T) {
	if got := loadJSON(t, `{"theme": "dracula"}`).Theme; got != settings.DraculaTheme {
		t.Errorf("Theme = %q, want %q", got, settings.DraculaTheme)
	}
	if got := loadJSON(t, `{"theme": "bogus"}`).Theme; got != settings.SystemTheme {
		t.Errorf("Theme = %q, want %q (unknown falls back to System)", got, settings.SystemTheme)
	}
}

func TestAddRecentMovesToFrontAndDedupes(t *testing.T) {
	s := settings.Settings{}
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t2.Add(time.Hour)

	s.AddRecent(settings.RecentRepo{Path: "/a"}, t1)
	s.AddRecent(settings.RecentRepo{Path: "/b"}, t2)
	s.AddRecent(settings.RecentRepo{Path: "/a"}, t3)

	want := []settings.RecentRepo{
		{Path: "/a", LastOpened: t3.Format(time.RFC3339)},
		{Path: "/b", LastOpened: t2.Format(time.RFC3339)},
	}
	if !slices.Equal(s.RecentRepos, want) {
		t.Errorf("RecentRepos = %+v, want %+v", s.RecentRepos, want)
	}
}

func TestAddRecentListsARepoOnceAcrossItsWorktrees(t *testing.T) {
	s := settings.Settings{}
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	s.AddRecent(settings.RecentRepo{Path: "/r/main", RepoKey: "/r/main/.git"}, t1)
	s.AddRecent(settings.RecentRepo{Path: "/r/linked", RepoKey: "/r/main/.git"}, t2)

	want := []settings.RecentRepo{{Path: "/r/linked", LastOpened: t2.Format(time.RFC3339), RepoKey: "/r/main/.git"}}
	if !slices.Equal(s.RecentRepos, want) {
		t.Errorf("RecentRepos = %+v, want %+v", s.RecentRepos, want)
	}
}

func TestAddRecentKeepsSameRepoKeyInTwoDistrosApart(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddRecent(settings.RecentRepo{Path: "/home/u/r", Distro: "Ubuntu", RepoKey: "/home/u/r/.git"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/home/u/r", Distro: "Debian", RepoKey: "/home/u/r/.git"}, now)

	if len(s.RecentRepos) != 2 {
		t.Errorf("RecentRepos = %+v, want one entry per distro", s.RecentRepos)
	}
}

func TestAddRecentCapsAtTen(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range 11 {
		s.AddRecent(settings.RecentRepo{Path: string(rune('a' + i))}, now.Add(time.Duration(i)*time.Hour))
	}

	if len(s.RecentRepos) != 10 {
		t.Fatalf("len(RecentRepos) = %d, want 10", len(s.RecentRepos))
	}
	if s.RecentRepos[0].Path != "k" {
		t.Errorf("most recent = %q, want %q", s.RecentRepos[0].Path, "k")
	}
	for _, r := range s.RecentRepos {
		if r.Path == "a" {
			t.Error("oldest entry \"a\" was not dropped")
		}
	}
}

func TestRemoveRecentDropsTheMatchingEntry(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddRecent(settings.RecentRepo{Path: "/a"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/b"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/c"}, now)

	s.RemoveRecent(settings.RecentRepo{Path: "/b"})

	want := []string{"/c", "/a"} // AddRecent inserts most-recent-first
	got := make([]string, len(s.RecentRepos))
	for i, r := range s.RecentRepos {
		got[i] = r.Path
	}
	if !slices.Equal(got, want) {
		t.Errorf("RecentRepos paths = %v, want %v", got, want)
	}
}

func TestRemoveRecentKeepsAnotherDistrosSamePathEntry(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddRecent(settings.RecentRepo{Path: "/r/main", RepoKey: "K"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/r/main", Distro: "Ubuntu", RepoKey: "K"}, now)

	s.RemoveRecent(settings.RecentRepo{Path: "/r/other-worktree", RepoKey: "K"})

	if len(s.RecentRepos) != 1 || s.RecentRepos[0].Distro != "Ubuntu" {
		t.Errorf("RecentRepos = %+v, want only the Ubuntu entry", s.RecentRepos)
	}
}

func TestAddRecentRoundTripsDistroOmittingItForLocal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := settings.Settings{}
	s.AddRecent(settings.RecentRepo{Path: "/wsl/path", Distro: "Ubuntu-22.04"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/local/path"}, now)

	if err := settings.Save(path, s); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk struct {
		RecentRepos []map[string]any `json:"recentRepos"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	// AddRecent inserts most-recent-first, so "/local/path" (added last) is
	// at index 0.
	if _, hasDistro := onDisk.RecentRepos[0]["distro"]; hasDistro {
		t.Errorf("local entry's JSON has a \"distro\" key, want it omitted: %+v", onDisk.RecentRepos[0])
	}

	got := load(t, path)
	want := []settings.RecentRepo{
		{Path: "/local/path", LastOpened: now.Format(time.RFC3339)},
		{Path: "/wsl/path", LastOpened: now.Format(time.RFC3339), Distro: "Ubuntu-22.04"},
	}
	if !slices.Equal(got.RecentRepos, want) {
		t.Errorf("RecentRepos = %+v, want %+v", got.RecentRepos, want)
	}
}

func TestStoreUpdateSavesAndGetReflectsTheChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.Update(func(s *settings.Settings) { s.DefaultPullMode = "rebase" }); err != nil {
		t.Fatal(err)
	}

	if got := st.Get().DefaultPullMode; got != "rebase" {
		t.Errorf("Get().DefaultPullMode = %q, want %q", got, "rebase")
	}
	if onDisk := load(t, path); onDisk.DefaultPullMode != "rebase" {
		t.Errorf("on-disk DefaultPullMode = %q, want %q", onDisk.DefaultPullMode, "rebase")
	}
}

func TestEditorProfileRoundTrips(t *testing.T) {
	want := settings.Settings{
		EditorProfile: core.EditorProfile{Name: "Alpha", FolderCmd: "alpha {path}", FileCmd: "alpha {path}", FileAtLineCmd: "alpha {path}:{line}"},
	}

	if got := saveAndLoad(t, want); got.EditorProfile != want.EditorProfile {
		t.Errorf("EditorProfile = %+v, want %+v", got.EditorProfile, want.EditorProfile)
	}
}

func TestLoadMigratesTheLocalEntryOfOldPerEnvironmentProfiles(t *testing.T) {
	got := loadJSON(t, `{"editorProfiles": {"": {"name": "Zed", "folderCmd": "zed {path}"}, "Ubuntu": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "Zed" {
		t.Errorf("EditorProfile = %+v, want Zed", got.EditorProfile)
	}
}

func TestLoadMigratesTheFirstFilledEntryWhenTheLocalOneIsEmpty(t *testing.T) {
	got := loadJSON(t, `{"editorProfiles": {"": {}, "Ubuntu": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "VS Code" {
		t.Errorf("EditorProfile = %+v, want VS Code", got.EditorProfile)
	}
}

func TestLoadKeepsTheSingleProfileOverOldEntries(t *testing.T) {
	got := loadJSON(t, `{"editorProfile": {"name": "Zed", "folderCmd": "zed {path}"}, "editorProfiles": {"": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "Zed" {
		t.Errorf("EditorProfile = %+v, want Zed", got.EditorProfile)
	}
}

// encoding/json ignores an older settings file's bare "editor" key.
func TestLoadIgnoresTheOldBareEditorStringKey(t *testing.T) {
	if got := loadJSON(t, `{"editor": "alpha"}`); got.EditorProfile != (core.EditorProfile{}) {
		t.Errorf("EditorProfile = %+v, want empty", got.EditorProfile)
	}
}
