# How each feature maps to git

Exact flags are fixed in the feature specs.

| Feature | Git |
|---|---|
| Open repo | `git rev-parse --path-format=absolute --show-toplevel --git-dir --git-common-dir --is-inside-work-tree` |
| Repo summary | `git symbolic-ref --short -q HEAD` (branch, detached HEAD); merge/rebase/cherry-pick/revert in progress from the worktree's own git dir, same file checks as "Operation in progress" below |
| Refs for labels | `git for-each-ref --format=%(refname)%1f%(objectname)%1f%(*objectname)%1f%(upstream:short)%1f%(upstream:track)%1f%(HEAD)%1f%(worktreepath)%00 refs/heads refs/remotes refs/tags` (`for-each-ref`'s own hex-escape syntax is `%1f`/`%00`, not `%x1f`/`%x00` like `git log`; `%(*objectname)` falls back to `%(objectname)` so an annotated tag's SHA is the commit it points to, not the tag object; `%(worktreepath)` marks which worktree has a *branch* checked out, for the sidebar's `CheckedOutIn` — FR-LOG-10, the log row's own checked-out marker, is a different fact, delivered via `worktree list --porcelain -z` instead, spec 10) |
| Log | `git log --topo-order --parents --decorate=full --exclude=refs/stash --all --format=<fields split by %x1f, %x00-terminated>` (no `-z`: git's own `%x00`/`\n` framing is enough, spec 10), streamed. `--exclude` before `--all` (order matters) drops `refs/stash`. `--decorate=full` gives ref labels their fully-qualified path (branch vs remote-tracking) |
| Search | `git log --topo-order --parents --decorate=full [--grep=<query>] [--author=<author>] [--regexp-ignore-case --fixed-strings] [--exclude=refs/stash --all] --format=<same fields as Log> --end-of-options [<branch>...] [-- <path>]`, streamed (spec 22, FR-LOG-05..08): `--grep`/`--author` combine with AND, not OR (git's own default across different predicate types — there is no flag that ORs them); `--fixed-strings` treats the query as a plain substring, not a regex; `--exclude=refs/stash --all` is omitted once explicit branches are given; `--end-of-options` guards the branch names (revisions) the same way it guards `<sha>` elsewhere, `--` guards the path. Hash: `git rev-parse --quiet --verify --end-of-options <query>^{commit}`; a hit fetches that one commit via `git log --decorate=full --format=<same fields> -1 --end-of-options <sha>` (`-1` before `--end-of-options`: git rejects a `-`-prefixed token placed after it as a bad revision) and short-circuits every other field. Text/author narrows to a flat list (no graph); branch/path filters keep the graph, `--parents` rewriting a path-filtered commit's parent across ones the filter drops. |
| Commit details | `git show --format=… --name-status -z -M --first-parent --end-of-options <sha>` (`--first-parent` is what diffs a merge against its first parent only, not all of them) |
| Diff file content | one file's two-sided content at a commit, first-parent semantics matching Commit details (FR-LOG-04): size cap via `git cat-file --batch-check='%(objectsize)'`, stdin `<sha>^:<old-or-path>\n<sha>:<path>\n`; binary classification via `git show --format= --numstat -z -M --first-parent --end-of-options <sha> -- <old-or-path> <path>` (git's own classification, not a NUL-byte sniff — respects `.gitattributes`, FR-DIFF-03); content via `git cat-file --batch` with the same stdin as the size check. A root commit's nonexistent parent and a missing side (add/delete) both read back as `<input> missing`, not an error |
| Compare commits | `git diff --name-status -z -M --end-of-options <a> <b>` (FR-CMP-01): the two-revision form (`a b`, i.e. `a..b`), never the three-dot merge-base form `a...b` — only the two-revision form makes swapping `a`/`b` mirror every file's status exactly (FR-CMP-03) |
| Compare range content (`DiffRange`, FR-CMP-02) | one file's content at two arbitrary commits: reuses "Diff file content"'s `cat-file --batch-check`/`--batch` calls with `<a>:<old-or-path>`/`<b>:<path>` in place of `<sha>^:<old-or-path>`/`<sha>:<path>`; binary classification via `git diff --numstat -z -M --end-of-options <a> <b> -- <old-or-path> <path>` (no `--format=` — a no-op on `git diff` between two commits, unlike `git show`) |
| Create branch | `git branch -- <name> <sha>`, optional `git switch <name>` |
| Reset | `git reset --soft\|--mixed\|--hard <sha>` (SHOULD: `--keep`) in the window's worktree; old HEAD recorded in the command log (FR-BR-04) |
| Checkout | `git switch <branch>` |
| Worktree list | `git worktree list --porcelain -z`; dirty flag via `git -C <path> status --porcelain=v2` |
| Worktree add | `git worktree add <path> <branch>` or `git worktree add -b <new> <path> <start>` |
| Worktree remove | `git worktree remove [--force] <path>`, `git worktree prune` |
| Status | `git status --porcelain=v2 -z --branch --untracked-files=all` |
| Ref tips | `git for-each-ref --format=%(refname)%1f%(objectname)%00 refs/heads refs/tags refs/remotes` (note: `for-each-ref`'s own hex-escape syntax is `%1f`/`%00`, not `%x1f`/`%x00` like `git log`), plus a synthetic `HEAD` entry: `git symbolic-ref --short -q HEAD` if on a branch, `git rev-parse -q --verify HEAD` if detached. The cheap "did anything move" signature the log's reload check (below) compares against its last-finished snapshot |
| Stage / unstage file | `git add -- <path>` / `git restore --staged -- <path>` |
| Stage / unstage hunk or lines | partial patch → `git apply --cached --recount [--reverse] -` |
| Discard | `git restore -- <path>`; hunks: `git apply --reverse -` |
| Commit / amend | `git commit -F -` / `git commit --amend -F -`. Amend warning (FR-CHG-06): `git merge-base --is-ancestor HEAD @{upstream}` |
| Fetch | `git fetch --prune --progress <remote>` or `--all` |
| Pull | `git pull --no-rebase` or `--rebase` (FR-SYNC-02); optional `--autostash` |
| Push | `git push --progress [-u <remote> <branch>]` |
| Force push | `git push --force-with-lease --force-if-includes` (FR-SYNC-05). Bare `--force-with-lease` only: the `=<ref>:<expect>` form silently disables `--force-if-includes`. Plain `--force` is never used. |
| Check out commit | `git switch --detach <sha>` |
| Cherry-pick | `git cherry-pick [-x] [-n] <sha>…`. Git applies in argument order, so the app's oldest-first sort (FR-CP-02) is load-bearing and has its own tests. Merge commits are refused before calling git (FR-CP-05). |
| Revert | `git revert [-n] <sha>…`, newest first. Merge commits refused as for cherry-pick. |
| Operation in progress | unmerged entries in status; `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge/`, `rebase-apply/`, `sequencer/` in the worktree's git dir. File checks, no subprocess. |
| Resolve / continue / abort | `git add -- <path>`; `git merge\|rebase\|cherry-pick\|revert --continue\|--skip\|--abort` |
| Commit-graph (large repo, first open) | `git commit-graph write --reachable`, after the load's done signal, once `LoadLog` has loaded ≥ 20k rows and `<git-common-dir>/objects/info/commit-graph` doesn't exist yet | spec 13, QA-PERF-02, ADR-4 exception: cuts `git log`'s time-to-first-record and peak memory at real scale; failure is logged, never surfaced; git keeps the file current itself during `gc`, nebegit never rewrites it |

### Update another branch in the background (FR-UPD)

Branch `B` with upstream `R/B`:

1. `B` not checked out in any worktree → `git fetch R B:B`. Git allows this as a fast-forward only and refuses (exit 128) when `B` is checked out in any worktree, so the app's own worktree check is a pre-check, not the safety net. Refusal → "diverged".
2. `B` checked out in another worktree → `git fetch R B`, then, if that worktree is clean, `git -C <that worktree> merge --ff-only R/B`. Else report.
3. `B` is this window's branch → Pull, not this feature.

Runs as a job. Result comes from comparing `B` before and after (FR-UPD-05).

### Log loading and graph

1. One `git log` process; records are read as they stream.
2. Lane assignment follows lazygit's streaming pipe model (`pkg/gui/presentation/graph`, MIT), ported into `internal/graph`, not imported. It consumes only the previous row's lanes and the current commit:
   - open lanes each wait for a sha; the commit takes the lane waiting for it, or a new one;
   - the first parent inherits the lane; other parents open new lanes;
   - a row stores its lane, pass-through lanes, and per edge: starts / continues / ends;
   - a parent not yet seen (filtered or `--all` loads) opens a fresh lane; tested separately;
   - a colour slot is reused only after its lane ended, never by row index, so colours stay stable while streaming;
   - lanes are capped (default 32); rows beyond the cap render a "+k" marker.
3. Rows are appended to an in-memory slice; the UI is told via `log:progress`. First rows show at once (FR-LOG-09, QA-PERF-02).
4. The UI requests row windows by index.
5. Path filter: `--parents` with a path rewrites parents, so the graph stays valid.
6. Text search shows a flat list without graph lines.
7. A new filter kills the running `git log` and starts a new one.
8. `repo:changed refs` reloads the refs map; the log reloads only if tips changed.

### Noticing outside changes (FR-REPO-04)

- The common git dir is watched with fs notifications (`internal/watch.RepoWatcher`, spec 12): `HEAD`, `refs/`, `packed-refs`, `index`, `worktrees/`, `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-*`, `sequencer/`. Per-worktree files live under `<git dir>/worktrees/<name>/`, so one watch covers all windows. `objects/`, `logs/`, `hooks/`, `info/` are not watched; a `*.lock` file is ignored (git's lockfile-then-rename pattern — the rename's target already classifies). fsnotify is primary; a 5 s low-frequency poll is an insurance fallback for what it misses (a dropped event, an exhausted watch count, a filesystem it doesn't support at all — e.g. WSL's 9p, decided per-platform in spec 30). Debounce ~300 ms: an `onChange` fires once per distinct scope touched in the window, not once per fs event.
- What changed maps to one of four scopes, delivered as a `"repo:changed"` event's `scope`:

  | Base name | Scope |
  |---|---|
  | `HEAD`, `packed-refs`, any path under `refs/` (any depth) | `refs` |
  | `index` | `index` |
  | `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge`, `rebase-apply`, `sequencer` | `state` |
  | `worktrees/` itself, or a direct child of it | `worktrees` |

- Working files are not watched recursively. Status runs on window focus, after every own action, and on a timer while focused: start at 1 s so poll interval plus status time stays within 3 s; back off when status gets slow and show "refreshing". Polls are marked as background in the command log (FR-UX-02). A working-file poll only ever reports its own window's `index` scope (FR-REPO-04's working-file half); the fs-watched `.git/index` reports `index` repo-wide (the staging half).
- The app's own writes also trigger the watcher; debounce plus reload-after-write makes that harmless.
- The log/graph reloads only if its ref tips actually changed (spec 12): a `refs` scope event re-checks the "Ref tips" row above against the snapshot recorded when the log last finished, and restarts the stream only on a difference — a cheap `for-each-ref`, not a re-walk.

### Jobs, progress, cancel

- Every remote or slow action is a job with an id.
- Progress comes from git's `--progress` lines on stderr.
- Cancel = cancel the context, kill the process, re-read repo state. If git left a lock or a half-done operation, the UI says so and offers abort (QA-SAFE-04).

### Auth

- Git uses what the user has. `GIT_TERMINAL_PROMPT=0` makes it fail fast instead of waiting for input.
- Auth failure → error with git's text plus the hint "log in once from your terminal".
- SSH keys with a passphrase and no agent fail. Documented as a known limit.

### External editor (FR-ED-*)

- `EditorService` has `OpenFolder(path)` and `OpenFile(path, line)`. All UI actions go through it (FR-ED-03).
- Profiles are stored per environment (Windows, each WSL distro) because the command runs in the core (FR-ED-11). Default everywhere: the editor.
- Profile: `{name, folderCmd, fileCmd, fileAtLineCmd?}`. Placeholders `{path}`, `{line}`, `{column}`. Missing `fileAtLineCmd` falls back to `fileCmd`.
- Presets are data, not code. Flags other than the editor's are unverified against the editors' docs.

| Preset | Folder | File at line |
|---|---|---|
| the editor (default) | `editor {path}` | `editor {path}:{line}` |
| another editor | `other {path}` | `code --goto {path}:{line}` |
| an IDE | `ide {path}` | `idea --line {line} {path}` |
| a third editor | `myed {path}` | `myed {path}:{line}` |
| Custom | user input | user input |

- Terminal editors need a terminal wrapper in the command (`<terminal> -e myed +{line} {path}`), supplied by the user as a custom command.
- The template is split into arguments, then placeholders are filled. No shell (TR-10).
- The process starts detached (TR-12); only start errors are captured.
- Fallback (FR-ED-10): `git var GIT_EDITOR` for files, run without the `GIT_EDITOR=true` override; the OS opener (`xdg-open` / `explorer` / `open`) for folders.
- "Test" (FR-ED-08) runs `folderCmd` on the current repo path and reports the result.
- Start error → `EDITOR_NOT_FOUND` with a link to the setting (FR-ED-06).
- UI labels use the profile name: "Open in {name}" (FR-ED-05).

---
