package main

import (
	"strings"
	"testing"
)

// TestADR_3_PrintArgsOnePerLine is case 4's mechanism: the owner compares
// this output against the argument they passed, to catch silent path
// translation (Design).
func TestADR_3_PrintArgsOnePerLine(t *testing.T) {
	var out strings.Builder
	printArgs([]string{"/home/user/project", "second"}, &out)

	want := "/home/user/project\nsecond\n"
	if out.String() != want {
		t.Errorf("printArgs output = %q, want %q", out.String(), want)
	}
}
