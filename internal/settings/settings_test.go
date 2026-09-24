package settings_test

import (
	"encoding/json"
	"io/fs"
	"maps"
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

func TestFR_UX_04_SaveThenLoadRoundTripsEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := settings.Settings{
		RecentRepos:              []settings.RecentRepo{{Path: "/a", LastOpened: "2026-09-21T00:00:00Z"}},
		OpenWorktrees:            []string{"/a", "/b"},
		EditorProfiles:           map[string]core.EditorProfile{"": {Name: "the editor", FolderCmd: "editor {path}"}},
		ProtectedBranches:        []string{"main"},
		DefaultPullMode:          "rebase",
		DiffMode:                 "side-by-side",
		AutoFetchIntervalMinutes: 5,
		Theme:                    "dark",
		IgnoreWhitespace:         true,
	}

	if err := settings.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
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
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	got, err := settings.Load(path)

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, settings.Default()) {
		t.Errorf("Load() = %+v, want Default() %+v", got, settings.Default())
	}
}

func TestFR_UX_06_ThemeRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := settings.Settings{Theme: "dark"}

	if err := settings.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", got.Theme, "dark")
	}
}

func TestFR_UX_06_LoadOnMissingThemeKeyDefaultsToFollowTheOS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "" {
		t.Errorf("Theme = %q, want empty (follow the OS)", got.Theme)
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

func TestFR_WT_13_AddRecentListsARepoOnceAcrossItsWorktrees(t *testing.T) {
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

func TestFR_WT_13_AddRecentKeepsSameRepoKeyInTwoDistrosApart(t *testing.T) {
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

	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
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
	onDisk, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.DefaultPullMode != "rebase" {
		t.Errorf("on-disk DefaultPullMode = %q, want %q", onDisk.DefaultPullMode, "rebase")
	}
}

// TestFR_ED_11_EditorProfilesRoundTripPerEnvironment round-trips one
// profile for local ("") and one for a WSL distro.
func TestFR_ED_11_EditorProfilesRoundTripPerEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := settings.Settings{
		EditorProfiles: map[string]core.EditorProfile{
			"":             {Name: "the editor", FolderCmd: "editor {path}", FileCmd: "editor {path}", FileAtLineCmd: "editor {path}:{line}"},
			"Ubuntu-22.04": {Name: "another editor", FolderCmd: "other {path}", FileCmd: "other {path}"},
		},
	}

	if err := settings.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got.EditorProfiles, want.EditorProfiles) {
		t.Errorf("EditorProfiles = %+v, want %+v", got.EditorProfiles, want.EditorProfiles)
	}
}

// TestFR_ED_11_LoadIgnoresTheOldBareEditorStringKey: an older settings
// file's bare "editor" key is ignored by encoding/json's unknown-field
// handling, with no migration to write.
func TestFR_ED_11_LoadIgnoresTheOldBareEditorStringKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"editor": "editor"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.EditorProfiles) != 0 {
		t.Errorf("EditorProfiles = %+v, want empty", got.EditorProfiles)
	}
}
