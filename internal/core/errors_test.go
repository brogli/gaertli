package core_test

import (
	"encoding/json"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestErrorsSecurity_ErrorCrossesTheBoundaryAsOneType(t *testing.T) {
	in := &core.Error{Code: core.NotARepo, Message: "not a git repository", GitStderr: "fatal: x", Command: "git rev-parse"}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"code":"NOT_A_REPO","message":"not a git repository","gitStderr":"fatal: x","command":"git rev-parse"}`
	if string(b) != want {
		t.Errorf("json = %s\nwant   %s", b, want)
	}

	var out core.Error
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out != *in {
		t.Errorf("round trip = %+v, want %+v", out, *in)
	}
	if got := in.Error(); got != "NOT_A_REPO: not a git repository" {
		t.Errorf("Error() = %q", got)
	}
}
