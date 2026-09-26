# Errors and security

## Errors

One error type crosses the core boundary, in-process and over RPC alike:
`{code, message, key, args, gitStderr, command}`. `code` is the machine-readable failure class; the
codes and their meanings are declared in `internal/core`. `key` names the message's wording, since
one code can read several ways (`GIT_FAILED` covers both "worktree already exists" and "can't
cherry-pick a merge commit"), and `args` holds its variable parts, so the UI can translate it
(FR-UX-11). `message` stays English: the app log's record, and the UI's fallback where `key` has no
translation or is empty (git's own message). The keys and their args are declared once in
`internal/core`.

`INTERNAL` shows a fixed headline with `message` behind "show details". Other errors put git's
stderr and command there (FR-UX-01).

Known failures are recognised from exit codes and machine-readable state, not by matching message
text. Where git gives no other signal (a failed sign-in, a held lock, a few refusals), its message
is matched; git's messages are pinned to English for that (TR-09).

## Security

- No shell anywhere, and repo text never becomes an option (TR-10).
- Branch and worktree names are validated with `git check-ref-format`.
- The app log (`components.md`) never records environment variables, and masks credentials embedded
  in a command's arguments or in a failed command's stderr.
- The WebView loads bundled assets only. Commit messages and file contents are rendered as text,
  never as HTML.
- No telemetry and no update check (QA-SEC-01).
