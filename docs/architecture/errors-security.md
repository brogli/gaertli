# Errors and security

## Errors

One error type crosses the core boundary, in-process and over RPC alike: `{code, message, key, args, gitStderr, command}`. `code` is the machine-readable failure class. `key` names the message's wording, since one code can read several ways (`GIT_FAILED` covers both "worktree already exists" and "can't cherry-pick a merge commit"), and `args` holds its variable parts, so the UI can translate it (FR-UX-11). `message` stays English: the app log's record and the UI's fallback where `key` has no translation yet or is empty (`Internal`, or git's own message).

`internal/core` keeps one table of every key and its args' names. A generated, checked-in JSON copy lets a Vitest test check the frontend's catalogue against it without running Go. Code outside `internal/core` changes `message` and `key` only through its constructors and `Retag`, so a message cannot drift from its key.

`Internal` shows a fixed headline with `message` behind "show details". Other errors put git's stderr and command there (FR-UX-01), except an editor start failure, whose message already names the command.

| Code | Meaning |
|---|---|
| `NOT_A_REPO` | the path is not inside a git worktree |
| `GIT_TOO_OLD` | git is older than TR-04 allows |
| `AUTH_FAILED` | sign-in to a remote failed |
| `PUSH_REJECTED` | the remote declined a push for a reason other than being behind |
| `NOT_FAST_FORWARD` | a push or branch update is not a fast-forward |
| `DIRTY_WORKTREE` | local changes are in the way |
| `CONFLICTS` | an operation stopped on conflicts, or cannot continue until they are resolved |
| `LOCKED` | another process holds a git lock |
| `INVALID_NAME` | a branch or worktree name fails `git check-ref-format` |
| `NO_UPSTREAM` | the branch has no upstream to update from |
| `UNMERGED_BRANCH` | deleting the branch would lose commits |
| `EDITOR_NOT_FOUND` | the editor command could not start |
| `FILE_NOT_FOUND` | the file to open no longer exists in the worktree |
| `CANCELLED` | the user cancelled the job |
| `GIT_FAILED` | any other git failure; git's own message is shown |
| `INTERNAL` | not git's failure: a bug, a bad request, a broken transport |
| `WSL_NOT_FOUND` | `wsl.exe` is missing, or WSL was asked for on a non-Windows shell |
| `DISTRO_NOT_FOUND` | the named WSL distro is unknown or not running |
| `DEPLOY_REFUSED` | the user declined copying the core into a distro |

Known failures are detected from exit codes and machine-readable state, not by matching message text. Git's messages are pinned to English (TR-09), which keeps the text stable for the few cases where matching cannot be avoided because git gives no other signal: sign-in failures, a non-fast-forward fetch, `branch -d` refusing an unmerged branch, `worktree remove` refusing a dirty worktree, and a pull's merge or rebase refusing to start over local changes.

## Security

- No shell anywhere, and repo text never becomes an option (TR-10).
- Branch and worktree names are validated with `git check-ref-format`.
- The app log (`components.md`) never records environment variables, and masks credentials embedded in a command's arguments or in a failed command's stderr.
- The WebView loads bundled assets only. Commit messages and file contents are rendered as text, never as HTML.
- No telemetry and no update check (QA-SEC-01).
