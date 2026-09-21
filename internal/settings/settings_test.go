package settings_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		Editor:                   "editor",
		ProtectedBranches:        []string{"main"},
		DefaultPullMode:          "rebase",
		DiffMode:                 "side-by-side",
		AutoFetchIntervalMinutes: 5,
	}

	if err := settings.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !equalSettings(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestFR_UX_04_LoadOnMissingFileReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	got, err := settings.Load(path)

	if err != nil {
		t.Fatal(err)
	}
	if !equalSettings(got, settings.Default()) {
		t.Errorf("Load() = %+v, want Default() %+v", got, settings.Default())
	}
}

func TestFR_REPO_02_AddRecentMovesToFrontAndDedupes(t *testing.T) {
	s := settings.Settings{}
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t2.Add(time.Hour)

	s.AddRecent("/a", t1)
	s.AddRecent("/b", t2)
	s.AddRecent("/a", t3)

	want := []settings.RecentRepo{
		{Path: "/a", LastOpened: t3.Format(time.RFC3339)},
		{Path: "/b", LastOpened: t2.Format(time.RFC3339)},
	}
	if !equalRecents(s.RecentRepos, want) {
		t.Errorf("RecentRepos = %+v, want %+v", s.RecentRepos, want)
	}
}

func TestFR_REPO_02_AddRecentCapsAtTen(t *testing.T) {
	s := settings.Settings{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range 11 {
		s.AddRecent(string(rune('a'+i)), now.Add(time.Duration(i)*time.Hour))
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

func TestStoreUpdateSavesAndGetReflectsTheChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.Update(func(s *settings.Settings) { s.Editor = "editor" }); err != nil {
		t.Fatal(err)
	}

	if got := st.Get().Editor; got != "editor" {
		t.Errorf("Get().Editor = %q, want %q", got, "editor")
	}
	onDisk, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Editor != "editor" {
		t.Errorf("on-disk Editor = %q, want %q", onDisk.Editor, "editor")
	}
}

func equalSettings(a, b settings.Settings) bool {
	return equalRecents(a.RecentRepos, b.RecentRepos) &&
		equalStrings(a.OpenWorktrees, b.OpenWorktrees) &&
		a.Editor == b.Editor &&
		equalStrings(a.ProtectedBranches, b.ProtectedBranches) &&
		a.DefaultPullMode == b.DefaultPullMode &&
		a.DiffMode == b.DiffMode &&
		a.AutoFetchIntervalMinutes == b.AutoFetchIntervalMinutes
}

func equalRecents(a, b []settings.RecentRepo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
