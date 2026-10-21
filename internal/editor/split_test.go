// Requirements: FR-ED-04, FR-ED-12

package editor

import (
	"slices"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"simple", "myed {path}", []string{"myed", "{path}"}, false},
		{"double-quoted token", `myed "{path}:{line}"`, []string{"myed", "{path}:{line}"}, false},
		{"backslash-escaped space", `myed\ +{line}`, []string{"myed +{line}"}, false},
		{"empty template", "", nil, true},
		{"unterminated quote", `"unterminated`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommand(tt.in, false)
			if tt.wantErr {
				if err == nil {
					t.Fatal("splitCommand: want an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitCommand(%q, false) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitCommandWindowsRule(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"unquoted path with backslashes", `C:\tools\ed.exe {path}`, []string{`C:\tools\ed.exe`, "{path}"}},
		{"quoted path with backslashes", `"C:\Program Files\My Editor\ed.exe" {path}`, []string{`C:\Program Files\My Editor\ed.exe`, "{path}"}},
		{"apostrophe in path", `C:\Users\O'Brien\ed.exe`, []string{`C:\Users\O'Brien\ed.exe`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommand(tt.in, true)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitCommand(%q, true) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
