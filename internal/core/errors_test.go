// Requirements: FR-UX-01, FR-UX-11

package core_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/brogli/gaertli/internal/core"
)

func TestErrorCrossesTheBoundaryAsOneType(t *testing.T) {
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
	if !reflect.DeepEqual(out, *in) {
		t.Errorf("round trip = %+v, want %+v", out, *in)
	}
	if got := in.Error(); got != "NOT_A_REPO: not a git repository" {
		t.Errorf("Error() = %q", got)
	}
}

func TestErrorArgsNameTheMessagesVariableParts(t *testing.T) {
	e := core.ErrorfArgs(core.EditorNotFound, "editorNotOnPath", map[string]string{"command": "myed"}, "%q is not on PATH", "myed")

	if want := `"myed" is not on PATH`; e.Message != want {
		t.Errorf("Message = %q, want %q", e.Message, want)
	}
	if e.Key != "editorNotOnPath" {
		t.Errorf("Key = %q, want editorNotOnPath", e.Key)
	}
	if e.Args["command"] != "myed" {
		t.Errorf("Args = %+v, want command=myed", e.Args)
	}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"code":"EDITOR_NOT_FOUND","message":"\"myed\" is not on PATH","key":"editorNotOnPath","args":{"command":"myed"}}`
	if string(b) != want {
		t.Errorf("json = %s\nwant   %s", b, want)
	}
}

func TestErrorfLeavesKeyAndArgsZero(t *testing.T) {
	e := core.Errorf(core.Internal, "boom")
	if e.Key != "" {
		t.Errorf("Key = %q, want empty", e.Key)
	}
	if e.Args != nil {
		t.Errorf("Args = %+v, want nil", e.Args)
	}
}

func TestRetagReclassifiesAnExistingError(t *testing.T) {
	e := &core.Error{Code: core.GitFailed, Message: "old", GitStderr: "fatal: x", Command: "git push"}

	e.Retag(core.PushRejected, "pushRejected", map[string]string{"reason": "hook declined"}, "The remote refused the push (%s).", "hook declined")

	if e.Code != core.PushRejected {
		t.Errorf("Code = %s, want %s", e.Code, core.PushRejected)
	}
	if want := "The remote refused the push (hook declined)."; e.Message != want {
		t.Errorf("Message = %q, want %q", e.Message, want)
	}
	if e.Key != "pushRejected" {
		t.Errorf("Key = %q, want pushRejected", e.Key)
	}
	if e.Args["reason"] != "hook declined" {
		t.Errorf("Args = %+v, want reason=hook declined", e.Args)
	}
	if e.GitStderr != "fatal: x" || e.Command != "git push" {
		t.Errorf("GitStderr/Command changed: %q/%q, want them untouched", e.GitStderr, e.Command)
	}
}

func TestErrorKeysHasNoEmptyKeyOrNilArgs(t *testing.T) {
	for key, args := range core.ErrorKeys {
		if key == "" {
			t.Error("ErrorKeys has an empty key")
		}
		if args == nil {
			t.Errorf("ErrorKeys[%q] = nil, want a (possibly empty) slice", key)
		}
	}
}
