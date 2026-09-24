package app

import (
	"os"
	"strings"
	"testing"
)

func TestQA_INST_01_GitPresent(t *testing.T) {
	realPath := os.Getenv("PATH")

	t.Setenv("PATH", t.TempDir())
	if gitPresent() {
		t.Error("gitPresent() = true with an empty PATH")
	}

	t.Setenv("PATH", realPath)
	if !gitPresent() {
		t.Error("gitPresent() = false with the real PATH")
	}
}

func TestQA_INST_01_Messages(t *testing.T) {
	if msg := text(missingGit, nil); !strings.Contains(msg, "git") {
		t.Errorf("missingGit = %q, want it to mention git", msg)
	}

	msg := text(startupFailed, map[string]string{"error": "x"})
	for _, want := range []string{"WebView2", "webkitgtk", "x"} {
		if !strings.Contains(msg, want) {
			t.Errorf("startupFailed = %q, want it to contain %q", msg, want)
		}
	}
}
