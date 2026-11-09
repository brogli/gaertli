# Backend

## Structure

The component diagram is in `architecture.md`.

- Dependencies point down: only the shell knows windows and Wails; below it everything is plain Go,
  so the WSL core builds without cgo.
- The core API is one interface per bounded context, composed into what a window may call and what
  only the shell calls; a context owns its calls, its types and its session state, and reads the
  others' only through their types.
- The operation a stop leaves in progress is one kind value; its detection and what the app refuses
  before git runs are functions of git's state files, tested as tables.
- The shell reaches repos only through the core API. It never runs git for a repo itself.
- Settings belong to the shell, also for WSL repos; each core gets the part it needs, such as the
  editor profile, when it connects. Opening a window is a shell action, since a core has no windows.
- The shell owns the one app log. A core reports its git commands and editor starts as events, so a
  WSL core's commands land in the same log and debug mode applies to every core at once.

## Calls and events

- **Calls** (UI → Go) go through generated bindings and return a result or the core error. Large
  data (log rows, file contents) always moves by call. A window can reach only the window part of
  the API; what the shell does for itself (opening sessions, configuring a core) is not bound. A
  value with a rule (a SHA, a full ref name, a filter naming refs) is checked once, where the call
  enters the shell, so no core sees a malformed one and git is never handed a revision expression or
  an option in its place.
- **Events** (Go → UI) carry small payloads and tell the UI to reload or update:

| Event              | Meaning                                                                 |
| ------------------ | ----------------------------------------------------------------------- |
| `repo:changed`     | something changed, in scope `refs`, `index`, `worktrees` or `state`     |
| `log:progress`     | more log rows are available, or the load failed                         |
| `job:update`       | a job started, progressed or ended                                      |
| `core:reconnected` | a WSL core restarted; windows on it reload                              |
| `settings:changed` | settings were saved; every window reloads them                          |
| `diff:show`        | the shell retargets an open diff window                                 |
| `command`,`editor` | a git command ran or an editor started; for the app log, never a window |

Payload types are declared once in Go and generated for TypeScript. The shell sends each event only
to the windows it concerns (ADR-8); a WSL core's events reach only windows on that distro.

## Errors

One error type crosses the core boundary, in-process and over RPC alike. It carries a failure class
for the code to act on, a message key and arguments for the UI to translate (one class reads several
ways), an English message for the log and as fallback, and git's command and stderr for the user to
inspect.

Failures are recognised from exit codes and the state git leaves, not from message text. Where git
gives no other signal (a failed sign-in, a held lock) its message is matched, which is why git's
messages run in English.
