package core

//go:generate go run ../../tools/errorkeysgen

// ErrorKeys lists every message key an ErrorfArgs or Retag call can
// produce, with the names of that message's args. en.json needs one
// errors.* entry per key; frontend/src/locales/errorKeys.generated.json
// mirrors this table for the frontend's tests.
var ErrorKeys = map[string][]string{
	"notARepo":                    {"path"},
	"noWorktreeLeft":              {"path"},
	"editorNotOnPath":             {"command"},
	"editorStartFailed":           {"command"},
	"editorCommandInvalid":        {"command"},
	"fileNotFound":                {"path"},
	"noUpstream":                  {"branch"},
	"branchCheckedOutDirty":       {"branch", "path"},
	"worktreeExists":              {"path"},
	"mergeCommitNotPickable":      {"sha", "verb"},
	"gitTooOld":                   {"version", "minVersion"},
	"deployRefused":               {"distro"},
	"pushFetchFirst":              {},
	"pushNotFastForward":          {},
	"pushStaleInfo":               {},
	"pushRefUpdatedSinceCheckout": {},
	"pushRejected":                {"reason"},
	"branchDiverged":              {"branch"},
	"authFailed":                  {},
	"invalidBranchName":           {"name"},
}
