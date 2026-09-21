package app

import (
	"errors"
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
	if !strings.Contains(missingGitMessage(), "git") {
		t.Errorf("missingGitMessage() = %q, want it to mention git", missingGitMessage())
	}

	msg := startupFailedMessage(errors.New("x"))
	for _, want := range []string{"WebView2", "webkitgtk", "x"} {
		if !strings.Contains(msg, want) {
			t.Errorf("startupFailedMessage() = %q, want it to contain %q", msg, want)
		}
	}
}
