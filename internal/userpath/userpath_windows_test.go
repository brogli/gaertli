//go:build windows

// Requirements: DIST-04

package userpath

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestAddAndRemoveEditPathUnderAGivenKeyAndKeepItsType(t *testing.T) {
	key := scratchKey(t)
	dir := `C:\Apps\gaertli\bin`
	add := func(path string) string { return withEntry(path, dir) }
	remove := func(path string) string { return withoutEntry(path, dir) }

	if err := key.SetExpandStringValue(pathValueName, `%USERPROFILE%\x`); err != nil {
		t.Fatalf("seeding Path: %v", err)
	}

	if err := updateKey(key, add); err != nil {
		t.Fatalf("first add: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x;C:\Apps\gaertli\bin`)

	if err := updateKey(key, add); err != nil {
		t.Fatalf("second add: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x;C:\Apps\gaertli\bin`)

	if err := updateKey(key, remove); err != nil {
		t.Fatalf("remove: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x`)

	if err := key.DeleteValue(pathValueName); err != nil {
		t.Fatalf("clearing Path: %v", err)
	}
	if err := updateKey(key, add); err != nil {
		t.Fatalf("add on a missing value: %v", err)
	}
	assertPath(t, key, dir)
}

// scratchKey creates a key under HKCU\Software for the test to edit,
// deleted when the test ends: never the real HKCU\Environment.
func scratchKey(t *testing.T) registry.Key {
	t.Helper()
	const name = `Software\gaertli-userpath-test`
	key, _, err := registry.CreateKey(registry.CURRENT_USER, name, registry.ALL_ACCESS)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, name); err != nil {
			t.Errorf("DeleteKey: %v", err)
		}
	})
	return key
}

func assertPath(t *testing.T, key registry.Key, want string) {
	t.Helper()
	got, typ, err := key.GetStringValue(pathValueName)
	if err != nil {
		t.Fatalf("GetStringValue: %v", err)
	}
	if typ != registry.EXPAND_SZ {
		t.Fatalf("Path type = %d, want EXPAND_SZ", typ)
	}
	if got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}
