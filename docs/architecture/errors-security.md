# Errors and security

- One error type crosses the boundary: `{code, message, gitStderr, command}`.
- Codes: `NOT_A_REPO`, `GIT_TOO_OLD`, `AUTH_FAILED`, `PUSH_REJECTED`, `NOT_FAST_FORWARD`, `DIRTY_WORKTREE`, `CONFLICTS`, `LOCKED`, `INVALID_NAME`, `EDITOR_NOT_FOUND`, `FILE_NOT_FOUND` (`OpenFile`'s path no longer exists in the worktree: a file deleted or renamed since the diff being viewed, FR-ED-02), `CANCELLED`, `GIT_FAILED`, `INTERNAL` (not git's failure: a panic, a bad request, a broken transport), `WSL_NOT_FOUND` (`wsl.exe` not on PATH, or any WSL call attempted on a non-Windows shell), `DISTRO_NOT_FOUND` (the named distro is unknown to `wsl.exe`), `DEPLOY_REFUSED` (the consent dialog to copy the core into a distro was declined).
- Known failures are detected from exit code and machine-readable state, not by matching message text. English messages (TR-09) keep the text stable for the few cases where matching is unavoidable — `DISTRO_NOT_FOUND` is one: `wsl.exe` gives no machine-readable way to tell an unknown distro from one that isn't running, so both fold into this one code.
- The UI always offers "show details" with raw stderr (FR-UX-01).

---

## Security

- No shell; argument lists; `--` before paths, `--end-of-options` before revisions (TR-10).
- Branch and worktree names are validated with `git check-ref-format`.
- The command log never records environment variables. URLs with embedded credentials are masked.
- The WebView loads bundled assets only. Commit messages and file contents are rendered as text, never as HTML.
- No telemetry, no update check (QA-SEC-01).

---
