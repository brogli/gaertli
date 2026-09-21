# Errors and security

- One error type crosses the boundary: `{code, message, gitStderr, command}`.
- Codes: `NOT_A_REPO`, `GIT_TOO_OLD`, `AUTH_FAILED`, `PUSH_REJECTED`, `NOT_FAST_FORWARD`, `DIRTY_WORKTREE`, `CONFLICTS`, `LOCKED`, `INVALID_NAME`, `EDITOR_NOT_FOUND`, `CANCELLED`, `GIT_FAILED`, `INTERNAL` (not git's failure: a panic, a bad request, a broken transport).
- Known failures are detected from exit code and machine-readable state, not by matching message text. English messages (TR-09) keep the text stable for the few cases where matching is unavoidable.
- The UI always offers "show details" with raw stderr (FR-UX-01).

---

## Security

- No shell; argument lists; `--` before paths, `--end-of-options` before revisions (TR-10).
- Branch and worktree names are validated with `git check-ref-format`.
- The command log never records environment variables. URLs with embedded credentials are masked.
- The WebView loads bundled assets only. Commit messages and file contents are rendered as text, never as HTML.
- No telemetry, no update check (QA-SEC-01).

---
