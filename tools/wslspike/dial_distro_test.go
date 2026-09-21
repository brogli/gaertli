package main

import (
	"runtime"
	"testing"
)

// TestADR_3_DistroDialRefusesOnNonWindows is dial_other.go's refusal
// message (task 3): the wsl.exe hop is Windows-only, no real wsl.exe needed
// to check it. On Windows, dial_windows.go's distroCommand builds the real
// wsl.exe invocation instead, which needs the owner's machine (Design).
func TestADR_3_DistroDialRefusesOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows builds a real wsl.exe command; exercised by the owner (spec 06)")
	}

	_, err := distroCommand("test-distro", "/remote/path")
	if err == nil || err.Error() != "wsl.exe dial is Windows-only" {
		t.Errorf("err = %v, want %q", err, "wsl.exe dial is Windows-only")
	}
}
