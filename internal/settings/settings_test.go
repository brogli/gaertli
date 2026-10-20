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

func TestTR_13_PathIsUnderTheOSUserConfigDir(t *testing.T) {
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

func TestFR_UX_04_SaveThenLoadRoundTripsEveryField(t *testing.T) {
	want := settings.Settings{
		RecentRepos:    []settings.RecentRepo{{Path: "/a", LastOpened: "2026-09-21T00:00:00Z"}},
		OpenWorktrees:  []string{"/a", "/b"},
		DiffWindowSize: &settings.WindowSize{Width: 900, Height: 600},
		Preferences: settings.Preferences{
			EditorProfile:            core.EditorProfile{Name: "Alpha", FolderCmd: "alpha {path}"},
			ProtectedBranches:        []string{"main"},
			DefaultPullMode:          core.PullRebase,
			DiffMode:                 settings.SideBySideDiff,
			AutoFetchIntervalMinutes: 5,
			Theme:                    settings.DarkTheme,
			IgnoreWhitespace:         true,
			DebugMode:                true,
			WorktreeFolders:          map[string]string{"": "/wt", "Ubuntu": "/home/u/wt"},
		},
	}

	if got := saveAndLoad(t, want); !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

// An install from before the modes were set has a 0755 directory and a
// 0644 file; its next save tightens both.
func TestTR_29_ASavedStoreHasAFileAndDirectoryOnlyItsOwnerCanRead(t *testing.T) {
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

func TestFR_UX_04_LoadOnMissingFileReturnsDefault(t *testing.T) {
	got := load(t, filepath.Join(t.TempDir(), "does-not-exist.json"))

	if !reflect.DeepEqual(got, settings.Default()) {
		t.Errorf("Load() = %+v, want Default() %+v", got, settings.Default())
	}
}

func TestFR_SYNC_06_AFreshInstallProtectsMainAndMaster(t *testing.T) {
	got := load(t, filepath.Join(t.TempDir(), "does-not-exist.json"))

	if want := []string{"main", "master"}; !slices.Equal(got.ProtectedBranches, want) {
		t.Errorf("ProtectedBranches = %q, want %q", got.ProtectedBranches, want)
	}
}

func TestFR_SYNC_06_AnExistingFileKeepsTheProtectedBranchesItStores(t *testing.T) {
	got := loadJSON(t, `{"protectedBranches": null}`)

	if len(got.ProtectedBranches) != 0 {
		t.Errorf("ProtectedBranches = %q, want none, as stored", got.ProtectedBranches)
	}
}

func TestFR_UX_06_ThemeRoundTrips(t *testing.T) {
	got := saveAndLoad(t, settings.Settings{Preferences: settings.Preferences{Theme: settings.DarkTheme}})

	if got.Theme != settings.DarkTheme {
		t.Errorf("Theme = %q, want %q", got.Theme, "dark")
	}
}

func TestFR_UX_06_LoadOnMissingThemeKeyDefaultsToFollowTheOS(t *testing.T) {
	got := loadJSON(t, `{}`)

	if got.Theme != "" {
		t.Errorf("Theme = %q, want empty (follow the OS)", got.Theme)
	}
}

func TestFR_UX_06_LoadAcceptsTheFourDarkThemesAndFallsBackToSystemOnAnUnknownValue(t *testing.T) {
	if got := loadJSON(t, `{"theme": "dracula"}`).Theme; got != settings.DraculaTheme {
		t.Errorf("Theme = %q, want %q", got, settings.DraculaTheme)
	}
	if got := loadJSON(t, `{"theme": "bogus"}`).Theme; got != settings.SystemTheme {
		t.Errorf("Theme = %q, want %q (unknown falls back to System)", got, settings.SystemTheme)
	}
}

func TestFR_REPO_02_AddRecentMovesToFrontAndDedupes(t *testing.T) {
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

func TestFR_WT_16_AddRecentListsARepoOnceAcrossItsWorktrees(t *testing.T) {
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

func TestFR_WT_16_AddRecentKeepsSameRepoKeyInTwoDistrosApart(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddRecent(settings.RecentRepo{Path: "/home/u/r", Distro: "Ubuntu", RepoKey: "/home/u/r/.git"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/home/u/r", Distro: "Debian", RepoKey: "/home/u/r/.git"}, now)

	if len(s.RecentRepos) != 2 {
		t.Errorf("RecentRepos = %+v, want one entry per distro", s.RecentRepos)
	}
}

func TestFR_REPO_02_AddRecentCapsAtTen(t *testing.T) {
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

func TestFR_REPO_08_RemoveRecentDropsTheMatchingEntry(t *testing.T) {
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

func TestFR_REPO_08_RemoveRecentKeepsAnotherDistrosSamePathEntry(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddRecent(settings.RecentRepo{Path: "/r/main", RepoKey: "K"}, now)
	s.AddRecent(settings.RecentRepo{Path: "/r/main", Distro: "Ubuntu", RepoKey: "K"}, now)

	s.RemoveRecent(settings.RecentRepo{Path: "/r/other-worktree", RepoKey: "K"})

	if len(s.RecentRepos) != 1 || s.RecentRepos[0].Distro != "Ubuntu" {
		t.Errorf("RecentRepos = %+v, want only the Ubuntu entry", s.RecentRepos)
	}
}

func TestFR_REPO_07_AddRecentRoundTripsDistroOmittingItForLocal(t *testing.T) {
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

func TestFR_ED_11_EditorProfileRoundTrips(t *testing.T) {
	want := settings.Settings{Preferences: settings.Preferences{
		EditorProfile: core.EditorProfile{Name: "Alpha", FolderCmd: "alpha {path}", FileCmd: "alpha {path}", FileAtLineCmd: "alpha {path}:{line}"},
	}}

	if got := saveAndLoad(t, want); got.EditorProfile != want.EditorProfile {
		t.Errorf("EditorProfile = %+v, want %+v", got.EditorProfile, want.EditorProfile)
	}
}

func TestFR_ED_11_LoadMigratesTheLocalEntryOfOldPerEnvironmentProfiles(t *testing.T) {
	got := loadJSON(t, `{"editorProfiles": {"": {"name": "Zed", "folderCmd": "zed {path}"}, "Ubuntu": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "Zed" {
		t.Errorf("EditorProfile = %+v, want Zed", got.EditorProfile)
	}
}

func TestFR_ED_11_LoadMigratesTheFirstFilledEntryWhenTheLocalOneIsEmpty(t *testing.T) {
	got := loadJSON(t, `{"editorProfiles": {"": {}, "Ubuntu": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "VS Code" {
		t.Errorf("EditorProfile = %+v, want VS Code", got.EditorProfile)
	}
}

func TestFR_ED_11_LoadKeepsTheSingleProfileOverOldEntries(t *testing.T) {
	got := loadJSON(t, `{"editorProfile": {"name": "Zed", "folderCmd": "zed {path}"}, "editorProfiles": {"": {"name": "VS Code", "folderCmd": "code {path}"}}}`)

	if got.EditorProfile.Name != "Zed" {
		t.Errorf("EditorProfile = %+v, want Zed", got.EditorProfile)
	}
}

// encoding/json ignores an older settings file's bare "editor" key.
func TestFR_ED_11_LoadIgnoresTheOldBareEditorStringKey(t *testing.T) {
	if got := loadJSON(t, `{"editor": "alpha"}`); got.EditorProfile != (core.EditorProfile{}) {
		t.Errorf("EditorProfile = %+v, want empty", got.EditorProfile)
	}
}
