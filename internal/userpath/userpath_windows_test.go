//go:build windows

package userpath

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestDIST_04_AddAndRemoveEditPathUnderAGivenKeyAndKeepItsType(t *testing.T) {
	key := scratchKey(t)
	dir := `C:\Apps\nebegit\bin`

	if err := key.SetExpandStringValue(pathValueName, `%USERPROFILE%\x`); err != nil {
		t.Fatalf("seeding Path: %v", err)
	}

	if err := addTo(key, dir); err != nil {
		t.Fatalf("first addTo: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x;C:\Apps\nebegit\bin`)

	if err := addTo(key, dir); err != nil {
		t.Fatalf("second addTo: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x;C:\Apps\nebegit\bin`)

	if err := removeFrom(key, dir); err != nil {
		t.Fatalf("removeFrom: %v", err)
	}
	assertPath(t, key, `%USERPROFILE%\x`)

	if err := key.DeleteValue(pathValueName); err != nil {
		t.Fatalf("clearing Path: %v", err)
	}
	if err := addTo(key, dir); err != nil {
		t.Fatalf("addTo on a missing value: %v", err)
	}
	assertPath(t, key, dir)
}

// scratchKey creates a key under HKCU\Software for the test to edit,
// deleted when the test ends: never the real HKCU\Environment.
func scratchKey(t *testing.T) registry.Key {
	t.Helper()
	const name = `Software\nebegit-userpath-test`
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
