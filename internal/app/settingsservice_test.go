package app

import (
	"context"
	"maps"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

func TestSettingsService_SetThenGetRoundTrips(t *testing.T) {
	reg, _ := newTestRegistry()
	s := NewSettingsService(newTestStore(t), reg)
	want := settings.Settings{
		EditorProfiles:  map[string]core.EditorProfile{"": {Name: "the editor", FolderCmd: "editor {path}"}},
		DefaultPullMode: "rebase",
		DiffMode:        "unified",
	}

	if err := s.Set(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got.EditorProfiles, want.EditorProfiles) || got.DefaultPullMode != want.DefaultPullMode {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

// An edit takes effect immediately wherever the profile is already in use,
// not just on the next connect.
func TestTR_13_SettingsServiceSetPushesTheNewProfileToTheLocalCore(t *testing.T) {
	reg, _ := newTestRegistry()
	var pushed []core.EditorProfile
	reg.SetLocal(fakeAPI{editorProfiles: &pushed})
	store := newTestStore(t)
	reg.SetStore(store)
	s := NewSettingsService(store, reg)

	next := settings.Settings{EditorProfiles: map[string]core.EditorProfile{"": {Name: "another editor", FolderCmd: "other {path}"}}}
	if err := s.Set(context.Background(), next); err != nil {
		t.Fatal(err)
	}

	if len(pushed) != 1 || pushed[0].Name != "another editor" {
		t.Errorf("pushed = %+v, want exactly one push of another editor", pushed)
	}
}
