# Errors and security

## Errors

One error type crosses the core boundary, in-process and over RPC alike: `{code, message, gitStderr, command}`. The UI shows the message and always offers the raw stderr and command behind "show details" (FR-UX-01).

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

Known failures are detected from exit codes and machine-readable state, not by matching message text. Git's messages are pinned to English (TR-09), which keeps the text stable for the few cases where matching cannot be avoided because git or `wsl.exe` gives no other signal: sign-in failures, a non-fast-forward fetch, `branch -d` refusing an unmerged branch, and `wsl.exe` not telling an unknown distro from one that is not running.

## Security

- No shell anywhere. Arguments are lists; `--` goes before paths and `--end-of-options` before revisions (TR-10).
- Branch and worktree names are validated with `git check-ref-format`.
- The command log never records environment variables, and masks credentials embedded in URLs.
- The WebView loads bundled assets only. Commit messages and file contents are rendered as text, never as HTML.
- No telemetry and no update check (QA-SEC-01).
