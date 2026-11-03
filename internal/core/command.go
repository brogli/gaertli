package core

// CommandRecord is one git command the core ran, streamed to the shell via
// the "command" event for the app log.
type CommandRecord struct {
	Command    string `json:"command"`
	Dir        string `json:"dir"`
	Background bool   `json:"background"`
	DurationMS int64  `json:"durationMs"`
	ExitCode   int    `json:"exitCode"`
	Stderr     string `json:"stderr,omitempty"`
	ErrorCode  Code   `json:"errorCode,omitempty"` // empty on success
}
