// Package userpath edits the Windows user PATH environment variable to add
// or remove one directory, keeping every other entry, its order and its
// %VAR% text exactly.
package userpath

import (
	"slices"
	"strings"
)

// withEntry appends dir to path unless an entry equal to it (case-insensitive,
// a trailing backslash ignored) is already there.
func withEntry(path, dir string) string {
	if hasEntry(path, dir) {
		return path
	}
	if path == "" {
		return dir
	}
	return path + ";" + dir
}

// withoutEntry drops every entry in path equal to dir (case-insensitive, a
// trailing backslash ignored), keeping every other entry, its order and its
// text.
func withoutEntry(path, dir string) string {
	entries := slices.DeleteFunc(strings.Split(path, ";"), func(entry string) bool { return sameEntry(entry, dir) })
	return strings.Join(entries, ";")
}

func hasEntry(path, dir string) bool {
	return slices.ContainsFunc(strings.Split(path, ";"), func(entry string) bool { return sameEntry(entry, dir) })
}

func sameEntry(entry, dir string) bool {
	return strings.EqualFold(strings.TrimSuffix(entry, `\`), strings.TrimSuffix(dir, `\`))
}
