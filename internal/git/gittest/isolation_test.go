package gittest

import (
	"strings"
	"testing"
)

func TestQA_PORT_01_IsolationErrorReportsWhenNotIsolated(t *testing.T) {
	env := map[string]string{}
	get := func(key string) string { return env[key] }

	if msg := isolationError(get); !strings.Contains(msg, "gittest.Main") {
		t.Errorf("isolationError(unset) = %q, want a message naming gittest.Main", msg)
	}

	env["GIT_CONFIG_NOSYSTEM"] = "1"
	env["GIT_CONFIG_GLOBAL"] = "/tmp/whatever/.gitconfig"
	if msg := isolationError(get); msg != "" {
		t.Errorf("isolationError(isolated) = %q, want \"\"", msg)
	}
}
