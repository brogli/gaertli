# How features map to git

Every feature is a small number of git calls in machine-readable formats (TR-08). A few rules apply to all of them:

- Arguments are passed as a list, never through a shell. `--` goes before paths and `--end-of-options` before revisions, so repo text never becomes an option (TR-10).
- Read-only calls run with `GIT_OPTIONAL_LOCKS=0`, so polling never takes the index lock away from the user's own commands (TR-09).
- Known failures are classified by exit code and repo state, not by message text, wherever git allows it (`errors-security.md`).
- Nothing is reimplemented that git already does: checkout of a remote branch uses git's own DWIM, branch names are validated by `git check-ref-format`, and pull is git's own fetch and merge/rebase.

| Feature | Git |
|---|---|
| Open repo | `rev-parse --show-toplevel --git-dir --git-common-dir` |
| Summary | `symbolic-ref HEAD`, `rev-parse HEAD`; operation state from files in the git dir (below) |
| Refs | `for-each-ref` over heads, remotes and tags, with upstream and ahead/behind |
| Log | `log --topo-order --parents --decorate=full --all`, streamed (below) |
| Search and filters | the same `log` with `--grep`, `--author`, branches and `-- <path>` |
| Commit details | `show --name-status -z -M --first-parent` |
| Diff content | `cat-file --batch` for both sides; binary detection via `--numstat`, which respects `.gitattributes` |
| Compare commits | `diff --name-status -z -M <a> <b>`, the two-revision form, so swapping mirrors the result |
| Compare with working tree | `diff --name-status -z -M <sha>` |
| Status | `status --porcelain=v2 -z --branch --untracked-files=all` |
| Stage, unstage | `add`, `restore --staged` |
| Stage lines | re-diff the path, build a partial patch, `apply --cached --recount` (ADR-7) |
| Discard | `restore --worktree`, `clean -f` for untracked files, `apply --reverse` for lines |
| Commit, amend | `commit [--amend] -F -`, the message on stdin; hooks run as usual |
| Create branch | `branch`, or `switch -c` when it is checked out at once |
| Checkout | `switch`, `switch --detach` for a commit |
| Reset | `reset --soft/--mixed/--hard/--keep` |
| Rename, delete branch | `branch -m`, `branch -d/-D` |
| Cherry-pick, revert | `cherry-pick`, `revert`, several commits in one call, in the order FR-CP-02/FR-RV-02 require |
| Continue, abort, skip | `<operation> --continue/--abort/--skip` |
| Worktrees | `worktree list --porcelain -z`, `worktree add/remove/prune` |
| Fetch | `fetch --prune --progress` |
| Pull | `fetch`, then `merge @{upstream}` or `rebase @{upstream}` in the worktree queue (ADR-6) |
| Stash and pull | `stash push`, pull, `stash pop` of exactly that entry, only after a clean integrate |
| Push | `push --progress --porcelain`; the porcelain summary classifies rejections |
| Force push | `push --force-with-lease --force-if-includes`, never plain `--force` |
| Commit-graph | `commit-graph write --reachable` once, on first open of a large repo (ADR-4) |

## Update another branch in the background (FR-UPD)

Updating a branch `B` from its upstream, without checking it out:

1. `B` is not checked out anywhere: `git fetch <remote> <upstream-ref>:refs/heads/B`. Git allows this only as a fast-forward and refuses it for a branch checked out in any worktree, so git itself is the safety net.
2. `B` is checked out in another worktree: that worktree must be clean. Fetch the remote-tracking ref only, then run `git merge --ff-only @{upstream}` in that worktree under its own write lock.
3. `B` is the current branch: that is Pull, not this feature.

Each update is its own job, so several branches can update at once. The result compares `B` before and after: updated by *n* commits, or already up to date.

## Log loading and graph

1. One `git log` process; records are read as they stream.
2. Lane assignment follows lazygit's streaming model, ported into `internal/graph`. It needs only the previous row's lanes and the current commit:
   - open lanes each wait for a sha; a commit takes the lane waiting for it, or a new one;
   - the first parent inherits the lane; other parents open new lanes;
   - a row stores its lane, the lanes passing through, and per edge whether it starts, continues or ends;
   - a parent not yet seen opens a fresh lane;
   - a colour is reused only after its lane ended, so colours stay stable while streaming;
   - lanes are capped (default 32); rows beyond the cap show a "+k" marker.
3. Rows are appended to an in-memory list and the UI is told through `log:progress`. The first rows show at once (FR-LOG-09, QA-PERF-02).
4. The UI requests row windows by index.
5. A path filter keeps the graph valid, because `--parents` rewrites parents across filtered-out commits. Text and author search show a flat list without graph lines.
6. A new filter kills the running filtered `git log` and starts a new one.
7. The log reloads only if ref tips changed (below).

## Noticing outside changes (FR-REPO-04)

The common git dir is watched with file-system notifications: `HEAD`, `refs/`, `packed-refs`, `index`, `worktrees/`, and the operation state files `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge/`, `rebase-apply/`, `sequencer/`. Per-worktree files live under `<git dir>/worktrees/<name>/`, so one watch covers all worktrees of a repo. Lock files are ignored, because git writes a lock file and renames it into place. A 5 s poll backs up the watcher for missed events and file systems without notifications. Events are debounced (~300 ms) and reported as one of four scopes:

| Changed | Scope |
|---|---|
| `HEAD`, `packed-refs`, anything under `refs/` | `refs` |
| `index` | `index` |
| operation state files | `state` |
| `worktrees/` or a direct child | `worktrees` |

Working files are not watched recursively. Instead, status runs on window focus, after every own action, and on a timer while the window is focused. The timer starts at 1 s, so an outside edit shows within 3 s, and backs off when status gets slow. Polls are marked as background in the command log (FR-UX-02).

The app's own writes also trigger the watcher. Debounce plus reload-after-write makes that harmless.

A `refs` change re-reads the ref tips (one `for-each-ref`) and restarts the log only if they differ from the snapshot taken when the log last finished.

## Jobs, progress, cancel

- Every remote or slow action is a job with an id: fetch, pull, push, force push, branch updates.
- Progress comes from git's `--progress` output on stderr. Git only reports progress to a terminal unless asked, so every job passes `--progress` explicitly. Parsing it is best effort: unknown wording shows as a plain status line.
- Cancel cancels the context and kills the process, then re-reads repo state. If git left a lock or a half-done operation behind, the UI says so and offers to abort (QA-SAFE-04).

## Auth

- Git uses whatever the user has set up. `GIT_TERMINAL_PROMPT=0` makes it fail fast instead of waiting for input.
- A sign-in failure is shown as `AUTH_FAILED` with git's text and the hint to log in once from a terminal.
- SSH keys with a passphrase and no agent fail. This is a documented limit.

## External editor (FR-ED)

- Every "open in editor" action goes through the core, so for a WSL repo the editor starts inside WSL. Profiles are therefore stored per environment: Windows and each WSL distro (FR-ED-11). The default everywhere is the editor.
- A profile is `{name, folderCmd, fileCmd, fileAtLineCmd?}` with the placeholders `{path}`, `{line}` and `{column}`. Without `fileAtLineCmd`, `fileCmd` is used.
- The template is split into arguments first, then the placeholders are filled in. No shell (TR-10).
- The editor starts detached (TR-12); only start errors are reported, as `EDITOR_NOT_FOUND` with a link to the setting.
- Presets are data in the settings panel:

| Preset | Folder / file | File at line |
|---|---|---|
| the editor (default) | `editor {path}` | `editor {path}:{line}` |
| another editor | `other {path}` | `other -g {path}:{line}` |
| an IDE | `ide {path}` | none, falls back to `ide {path}` |
| a third editor | `myed {path}` | `myed {path}:{line}` |
| a terminal editor (terminal) | `x-terminal-emulator -e myed {path}` | `x-terminal-emulator -e myed +{line} {path}` |
| Custom | user input | user input |

- "Test" (FR-ED-08) runs the profile's folder command on the home directory, in the environment the profile belongs to.
- Fallbacks (FR-ED-10), offered after `EDITOR_NOT_FOUND`: git's own editor (`git var GIT_EDITOR`), or the system opener (`explorer` on Windows, `xdg-open` on Linux and inside WSL).
