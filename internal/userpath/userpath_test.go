package userpath

import (
	"strings"
	"testing"
)

func TestDIST_04_WithEntryAppendsOnceAndKeepsOtherEntriesVerbatim(t *testing.T) {
	path := `C:\a;%USERPROFILE%\b`
	dir := `C:\Apps\nebegit\bin`

	once := withEntry(path, dir)
	twice := withEntry(once, dir)

	want := `C:\a;%USERPROFILE%\b;C:\Apps\nebegit\bin`
	if once != want {
		t.Fatalf("withEntry(%q, %q) = %q, want %q", path, dir, once, want)
	}
	if twice != once {
		t.Fatalf("a second withEntry changed the result: %q -> %q", once, twice)
	}
}

func TestDIST_04_WithEntryKeepsEveryCharacterOfALongPath(t *testing.T) {
	long := strings.Repeat(`C:\dir;`, 300) // 2100 characters
	dir := `C:\Apps\nebegit\bin`

	got := withEntry(long, dir)

	want := long + ";" + dir
	if got != want {
		t.Fatalf("withEntry on a %d-character path dropped or changed characters", len(long))
	}
}

func TestDIST_04_WithEntryIgnoresCaseAndATrailingBackslash(t *testing.T) {
	path := `C:\a;C:\APPS\NEBEGIT\BIN\;C:\b`
	dir := `C:\Apps\nebegit\bin`

	got := withEntry(path, dir)

	if got != path {
		t.Fatalf("withEntry(%q, %q) = %q, want it unchanged", path, dir, got)
	}
}

func TestDIST_04_WithoutEntryDropsEveryMatchAndKeepsTheRestInOrder(t *testing.T) {
	path := `C:\a;C:\Apps\nebegit\bin;C:\b;C:\APPS\NEBEGIT\BIN\;C:\c`
	dir := `C:\Apps\nebegit\bin`

	got := withoutEntry(path, dir)

	want := `C:\a;C:\b;C:\c`
	if got != want {
		t.Fatalf("withoutEntry(%q, %q) = %q, want %q", path, dir, got, want)
	}
}

func TestDIST_04_WithoutEntryOnAnEmptyPathStaysEmpty(t *testing.T) {
	if got := withoutEntry("", `C:\Apps\nebegit\bin`); got != "" {
		t.Fatalf(`withoutEntry("", dir) = %q, want ""`, got)
	}
}
