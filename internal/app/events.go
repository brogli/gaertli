package app

import (
	"encoding/json"
	"errors"

	"github.com/brogli/nebegit/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DiffShow is "diff:show"'s payload: what an open diff window shows next,
// and in which worktree.
type DiffShow struct {
	Wt     core.WorktreeID `json:"wt"`
	Target DiffTarget      `json:"target"`
}

// init registers every event name with its payload type; the binding
// generator turns them into the frontend's event types. Each name is also a
// key of eventPayloads.
func init() {
	application.RegisterEvent[core.RepoChanged]("repo:changed")
	application.RegisterEvent[core.LogProgress]("log:progress")
	application.RegisterEvent[core.JobInfo]("job:update")
	application.RegisterEvent[core.CommandRecord]("command")
	application.RegisterEvent[core.EditorRecord]("editor")
	application.RegisterEvent[application.Void]("core:reconnected")
	application.RegisterEvent[DiffShow]("diff:show")
	application.RegisterEvent[application.Void]("settings:changed")
}

// eventPayloads decodes a core.Event's JSON payload into the data type its
// name is registered with: Wails emits only data of exactly that type.
var eventPayloads = map[string]func(json.RawMessage) (any, error){
	"repo:changed":     decodePayload[core.RepoChanged],
	"log:progress":     decodePayload[core.LogProgress],
	"job:update":       decodePayload[core.JobInfo],
	"command":          decodePayload[core.CommandRecord],
	"editor":           decodePayload[core.EditorRecord],
	"core:reconnected": noPayload,
	"diff:show":        decodePayload[DiffShow],
	"settings:changed": noPayload,
}

// decodeEvent returns ev's payload as its registered data type.
func decodeEvent(ev core.Event) (any, error) {
	decode, ok := eventPayloads[ev.Name]
	if !ok {
		return nil, errors.New("unregistered event")
	}
	return decode(ev.Payload)
}

func decodePayload[T any](payload json.RawMessage) (any, error) {
	var data T
	err := json.Unmarshal(payload, &data)
	return data, err
}

// noPayload is the data of an event registered as application.Void.
func noPayload(json.RawMessage) (any, error) { return nil, nil }
