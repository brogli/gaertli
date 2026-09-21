package core

// CommandLogEntry is one command-log row (FR-UX-02): a git.Runner call as
// buffered by a session and streamed to the shell via the "command" event.
type CommandLogEntry struct {
	At         string `json:"at"` // RFC3339Nano
	Command    string `json:"command"`
	Dir        string `json:"dir"`
	Background bool   `json:"background"`
	DurationMS int64  `json:"durationMs"`
	ExitCode   int    `json:"exitCode"`
	Stderr     string `json:"stderr,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"` // core.Code, empty on success
}
