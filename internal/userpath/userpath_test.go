// Requirements: DIST-04

package userpath

import (
	"strings"
	"testing"
)

func TestWithEntryAppendsOnceAndKeepsOtherEntriesVerbatim(t *testing.T) {
	path := `C:\a;%USERPROFILE%\b`
	dir := `C:\Apps\gaertli\bin`

	once := withEntry(path, dir)
	twice := withEntry(once, dir)

	want := `C:\a;%USERPROFILE%\b;C:\Apps\gaertli\bin`
	if once != want {
		t.Fatalf("withEntry(%q, %q) = %q, want %q", path, dir, once, want)
	}
	if twice != once {
		t.Fatalf("a second withEntry changed the result: %q -> %q", once, twice)
	}
}

func TestWithEntryKeepsEveryCharacterOfALongPath(t *testing.T) {
	long := strings.Repeat(`C:\dir;`, 300) // 2100 characters
	dir := `C:\Apps\gaertli\bin`

	got := withEntry(long, dir)

	want := long + ";" + dir
	if got != want {
		t.Fatalf("withEntry on a %d-character path dropped or changed characters", len(long))
	}
}

func TestWithEntryIgnoresCaseAndATrailingBackslash(t *testing.T) {
	path := `C:\a;C:\APPS\GAERTLI\BIN\;C:\b`
	dir := `C:\Apps\gaertli\bin`

	got := withEntry(path, dir)

	if got != path {
		t.Fatalf("withEntry(%q, %q) = %q, want it unchanged", path, dir, got)
	}
}

func TestWithoutEntryDropsEveryMatchAndKeepsTheRestInOrder(t *testing.T) {
	path := `C:\a;C:\Apps\gaertli\bin;C:\b;C:\APPS\GAERTLI\BIN\;C:\c`
	dir := `C:\Apps\gaertli\bin`

	got := withoutEntry(path, dir)

	want := `C:\a;C:\b;C:\c`
	if got != want {
		t.Fatalf("withoutEntry(%q, %q) = %q, want %q", path, dir, got, want)
	}
}

func TestWithoutEntryOnAnEmptyPathStaysEmpty(t *testing.T) {
	if got := withoutEntry("", `C:\Apps\gaertli\bin`); got != "" {
		t.Fatalf(`withoutEntry("", dir) = %q, want ""`, got)
	}
}
