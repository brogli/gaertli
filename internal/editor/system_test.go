package editor

import (
	"strings"
	"testing"
)

func TestFR_ED_10_SystemProfileHasAPathPlaceholderAndAName(t *testing.T) {
	profile := SystemProfile()

	if profile.Name == "" {
		t.Error("SystemProfile().Name is empty")
	}
	if !strings.Contains(profile.FolderCmd, "{path}") {
		t.Errorf("SystemProfile().FolderCmd = %q, want it to contain {path}", profile.FolderCmd)
	}
	if !strings.Contains(profile.FileCmd, "{path}") {
		t.Errorf("SystemProfile().FileCmd = %q, want it to contain {path}", profile.FileCmd)
	}
}
