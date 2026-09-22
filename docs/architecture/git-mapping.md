# How each feature maps to git

Exact flags are fixed in the feature specs.

| Feature | Git |
|---|---|
| Open repo | `git rev-parse --path-format=absolute --show-toplevel --git-dir --git-common-dir --is-inside-work-tree` |
| Repo summary | `git symbolic-ref --short -q HEAD` (branch, detached HEAD); merge/rebase/cherry-pick/revert in progress from the worktree's own git dir, same file checks as "Operation in progress" below |
| Summary's HEAD SHA | `git rev-parse -q --verify --end-of-options HEAD` (QA-REL-02): exit 1 with empty stdout on an unborn HEAD (a fresh repo, no commits) reads back as `""`, not an error — the same shape "Repo summary"'s own detached-HEAD check already relies on. Lets "create branch here" (FR-BR-08) offer a start point whether attached or detached |
| Refs for labels | `git for-each-ref --format=%(refname)%1f%(objectname)%1f%(*objectname)%1f%(upstream:short)%1f%(upstream:track)%1f%(HEAD)%1f%(worktreepath)%00 refs/heads refs/remotes refs/tags` (`for-each-ref`'s own hex-escape syntax is `%1f`/`%00`, not `%x1f`/`%x00` like `git log`; `%(*objectname)` falls back to `%(objectname)` so an annotated tag's SHA is the commit it points to, not the tag object; `%(worktreepath)` marks which worktree has a *branch* checked out, for the sidebar's `CheckedOutIn` — FR-LOG-10, the log row's own checked-out marker, is a different fact, delivered via `worktree list --porcelain -z` instead) |
| Log | `git log --topo-order --parents --decorate=full --exclude=refs/stash --all --format=<fields split by %x1f, %x00-terminated>` (no `-z`: git's own `%x00`/`\n` framing is enough), streamed. `--exclude` before `--all` (order matters) drops `refs/stash`. `--decorate=full` gives ref labels their fully-qualified path (branch vs remote-tracking) |
| Search | `git log --topo-order --parents --decorate=full [--grep=<query>] [--author=<author>] [--regexp-ignore-case --fixed-strings] [--exclude=refs/stash --all] --format=<same fields as Log> --end-of-options [<branch>...] [-- <path>]`, streamed (FR-LOG-05..08): `--grep`/`--author` combine with AND, not OR (git's own default across different predicate types — there is no flag that ORs them); `--fixed-strings` treats the query as a plain substring, not a regex; `--exclude=refs/stash --all` is omitted once explicit branches are given; `--end-of-options` guards the branch names (revisions) the same way it guards `<sha>` elsewhere, `--` guards the path. Hash: `git rev-parse --quiet --verify --end-of-options <query>^{commit}`; a hit fetches that one commit via `git log --decorate=full --format=<same fields> -1 --end-of-options <sha>` (`-1` before `--end-of-options`: git rejects a `-`-prefixed token placed after it as a bad revision) and short-circuits every other field. Text/author narrows to a flat list (no graph); branch/path filters keep the graph, `--parents` rewriting a path-filtered commit's parent across ones the filter drops. |
| Commit details | `git show --format=… --name-status -z -M --first-parent --end-of-options <sha>` (`--first-parent` is what diffs a merge against its first parent only, not all of them) |
| Diff file content | one file's two-sided content at a commit, first-parent semantics matching Commit details (FR-LOG-04): size cap via `git cat-file --batch-check='%(objectsize)'`, stdin `<sha>^:<old-or-path>\n<sha>:<path>\n`; binary classification via `git show --format= --numstat -z -M --first-parent --end-of-options <sha> -- <old-or-path> <path>` (git's own classification, not a NUL-byte sniff — respects `.gitattributes`, FR-DIFF-03); content via `git cat-file --batch` with the same stdin as the size check. A root commit's nonexistent parent and a missing side (add/delete) both read back as `<input> missing`, not an error |
| Compare commits | `git diff --name-status -z -M --end-of-options <a> <b>` (FR-CMP-01): the two-revision form (`a b`, i.e. `a..b`), never the three-dot merge-base form `a...b` — only the two-revision form makes swapping `a`/`b` mirror every file's status exactly (FR-CMP-03) |
| Compare range content (`DiffRange`, FR-CMP-02) | one file's content at two arbitrary commits: reuses "Diff file content"'s `cat-file --batch-check`/`--batch` calls with `<a>:<old-or-path>`/`<b>:<path>` in place of `<sha>^:<old-or-path>`/`<sha>:<path>`; binary classification via `git diff --numstat -z -M --end-of-options <a> <b> -- <old-or-path> <path>` (no `--format=` — a no-op on `git diff` between two commits, unlike `git show`) |
| Working diff, staged (`DiffWorking`, FR-CHG-04) | one file's content, HEAD vs the index: reuses "Diff file content"'s `cat-file --batch-check`/`--batch` calls with `HEAD:<old-or-path>`/`:<path>` in place of `<sha>^:<old-or-path>`/`<sha>:<path>` (`:<path>` is git's own object spec for the index); binary classification via `git diff --cached --numstat -z -M -- <old-or-path> <path>` |
| Working diff, unstaged/untracked (`DiffWorking`, FR-CHG-04) | one file's content, the index (or nothing, if untracked) vs the literal working-tree file: the index side alone via `cat-file --batch-check`/`--batch` with stdin `:<path>`; the working-tree side is a plain filesystem read (`os.Stat`/`os.ReadFile`), not a git object until staged. Binary classification: a tracked path via `git diff --numstat -z -M -- <path>` (index vs worktree); an untracked path via `git diff --no-index --numstat -z -- /dev/null <path>` (no index side to compare against) — the one call in this codebase that exits 1 on a real difference, tolerated like `headSHA`'s unborn-HEAD exit 1 |
| Validate branch name | `git check-ref-format --branch <name>` (FR-BR-01): exit code alone (not stderr text) classifies `Code InvalidName`, before any mutating call. Also required before "Worktree add, new branch" below (FR-WT-02) — `worktree add -b`'s own `<name>` argument is not protected by `--`, unlike `switch -c`'s |
| Create branch, no checkout | `git branch -- <name> <start>` (FR-BR-01) |
| Create branch, with checkout | `git switch -c <name> --end-of-options <start>` (FR-BR-01): one atomic call — a checkout failure (e.g. local changes in the way) creates no orphan branch, unlike a separate `branch` + `switch` pair |
| Reset | `git reset --soft\|--mixed\|--hard <sha>` (SHOULD: `--keep`, still open) in the window's worktree (FR-BR-02): no `--end-of-options` (`reset` rejects it outright) and no `--` (silently reinterprets the target as a pathspec, leaving HEAD unchanged) — safe without either because `sha` is always an already-resolved commit hash, never user text. `--soft` uniquely refuses, exit 128, during any unresolved conflict (merge or rebase alike); `--mixed`/`--hard` are never refused for a dirty tree or a conflict in progress — both silently end an in-progress merge. The pre-reset HEAD is read via `rev-parse` before the call (FR-BR-04), not via `ORIG_HEAD` read back after |
| Uncommitted count | `git status --porcelain=v2 -z --untracked-files=no --no-renames`, read-only (FR-BR-03): `--no-renames`/`--untracked-files=no` together guarantee one NUL-terminated field per tracked change, so the raw NUL count is the number of tracked paths with an uncommitted change — untracked files excluded |
| Checkout (local or remote-tracking) | `git switch --end-of-options <branch>` (FR-BR-05): git's own DWIM turns a remote-tracking branch with no local branch of that name yet into a new local branch tracking it — no app-side branch-listing code. A "would be overwritten" refusal (local changes in the way) is exit 1, classified `Code DirtyWorktree`; every other refusal (invalid/nonexistent branch, ambiguous multi-remote match, already checked out in another worktree) is exit 128, plain `GitFailed` with git's own message |
| Check out commit (detached) | `git switch --detach --end-of-options <sha>` (FR-BR-08): same exit-1/exit-128 refusal split as Checkout. Unlike a branch, a detached commit may be checked out in several worktrees at once, so it is never refused for being "already checked out" |
| Worktree list | `git worktree list --porcelain -z` (FR-WT-01): each record NUL-framed, the main worktree always listed first (git's own ordering, the only signal for `Main`); `locked`/`prunable` fields |
| Worktree dirty check | `git -C <path> status --porcelain=v2 -z --untracked-files=all`, read-only, one worktree at a time (FR-WT-01): reuses "Status"'s own exact flags below |
| List branches (panel) | `git for-each-ref --format=%(refname:short)%1f%(worktreepath)%00 refs/heads` (FR-WT-13): a repo's local branches and which worktree, if any, has each checked out — same `%1f`/`%00` escape convention as "Refs for labels"/"Ref tips" below, but no upstream/ahead-behind data (that needs an already-open `WorktreeID`; the panel's own lightweight list does not) |
| Worktree add, existing branch | `git worktree add -- <target> <branch>` (FR-WT-02): `--` protects both positionals; `target` must not exist or must be empty — git's own refusal (`fatal: '<path>' already exists`), no app-side pre-check |
| Worktree add, new branch | `git worktree add -b <name> -- <target> <start>` (FR-WT-02, FR-WT-03): `<name>` is validated first ("Validate branch name" above) — its own argument is not protected by `--` |
| Worktree remove | `git worktree remove [--force] <path>`, `git worktree prune` |
| Status | `git status --porcelain=v2 -z --branch --untracked-files=all` (FR-CHG-01 reuses this exact row, unchanged — one canonical status-call shape, not a second near-identical one) |
| Ref tips | `git for-each-ref --format=%(refname)%1f%(objectname)%00 refs/heads refs/tags refs/remotes` (note: `for-each-ref`'s own hex-escape syntax is `%1f`/`%00`, not `%x1f`/`%x00` like `git log`), plus a synthetic `HEAD` entry: `git symbolic-ref --short -q HEAD` if on a branch, `git rev-parse -q --verify HEAD` if detached. The cheap "did anything move" signature the log's reload check (below) compares against its last-finished snapshot |
| Stage / unstage file | `git add -- <path>` / `git restore --staged -- <path>` |
| Stage / unstage all (FR-CHG-02) | `git add -A` / `git restore --staged -- .` (`.` over a bare `git reset`: identical result, no incidental human-readable stdout landing in the command log) |
| Stage / unstage lines (FR-CHG-03) | re-diff `diff -U3 -- <path>` (stage: old=index, new=working tree) or `diff --cached -U3 -- <path>` (unstage: old=HEAD, new=index, since unstaging peels lines off what's already staged), parsed fresh each call; a `LineSelection` names the changed lines to keep by their own 1-based line number on the diff's old/new side, not by a hunk index — each fragment folds its unselected changed lines into context or drops them, fragments with no selected line are left out entirely → `git apply --cached --recount [--reverse] -`. The UI offers this only for a plain modification (`ChangedFile.Status == "M"`); an untracked, added, deleted, or renamed file's whole-file `StageFile`/`UnstageFile` above is used instead, since the re-diff either lists no file (untracked) or produces a patch `git apply` refuses (added/deleted/renamed). An empty re-diff still reaching this call is `Code Internal`, naming the path |
| Discard | `git restore -- <path>`; hunks: `git apply --reverse -` |
| Commit / amend | `git commit -F -` / `git commit --amend -F -` (FR-CHG-05, FR-CHG-06), message always over stdin, never `-m`: no editor is ever invoked on either path, and a multi-line message survives byte for byte with no command-line injection surface. No `--no-edit`: `-F -` alone already supplies the message on the amend path. Amend succeeds with nothing staged on a clean tree; both refuse cleanly (`GitFailed`) for "nothing to commit", an empty message, and amend on an unborn HEAD. A failing hook's own output, on either stream, lands in the outer process's stderr (`GitStderr`), whichever hook (`pre-commit`, `commit-msg`, …) wrote it — no `HookFailed` code (FR-CHG-07). Amend warning (FR-CHG-06): the current branch's `Refs.Ahead == 0` against a configured, non-gone upstream — mathematically equivalent to `git merge-base --is-ancestor HEAD @{upstream}`, read from the already-loaded `Refs` instead of a new git call |
| Fetch | `git fetch --prune --progress <remote>` or `--all` when `remote == ""` (FR-SYNC-01). Runs outside the write queues (ADR-6): touches only refs/objects, never blocks or is blocked by a concurrent stage/commit/checkout |
| Pull | Not `git pull` (a single call would hold some lock scope for the whole network fetch, the exact thing ADR-6 exists to avoid). Two steps, one job (FR-SYNC-02): `git fetch --prune --progress`, no remote argument — follows `branch.<current>.remote`, git's own default, holding no lock — then, under `wt.Lock()`, `git merge --no-edit @{upstream}` (merge mode) or `git rebase @{upstream}` (rebase mode). A stop mid-way (`MERGE_HEAD`, or `rebase-merge`/`rebase-apply`, left behind) is `Code Conflicts`, left in place for the resolve flow below |
| Push | `git push --progress --porcelain [--set-upstream <remote> HEAD]`. `HEAD` as the source sets up tracking against a same-named branch on the far side, no extra call to resolve the current branch's own name. `--porcelain`: a rejected ref's own machine-readable summary line, always on stdout even on a failed call, classifies the rejection (FR-SYNC-04) — `Code NotFastForward` for a `(fetch first)`/`(stale info)`/`(remote ref updated since checkout)`/`(non-fast-forward)` reason, `Code PushRejected` for any other explained one (e.g. a server-side hook decline), unchanged `GitFailed` when no per-ref line printed at all (refused before it, e.g. no upstream configured) |
| Force push | `git push --progress --porcelain --force-with-lease --force-if-includes` (FR-SYNC-05). Bare `--force-with-lease` only: the `=<ref>:<expect>` form silently disables `--force-if-includes`. Plain `--force` is never used. Rejection classified the same way as Push's |
| Sign-in failure (fetch/pull/push/force push) | `GIT_TERMINAL_PROMPT=0` (unconditional) makes an interactive credential prompt structurally impossible, so a failed sign-in surfaces only as one of two exit-128 stderr shapes, matched by prefix (FR-SYNC-08, QA-SEC-02, same message-matching stance as `DISTRO_NOT_FOUND`): `fatal: could not read Username for` (git refuses before even attempting the request) or `fatal: Authentication failed for` (a credential was offered and rejected) — either classifies `Code AuthFailed`, its message pointing at the user's own git credential setup and naming no URL or credential text |
| Resolve candidates | `git log --no-walk --format=%H%x1f%h%x1f%ct%x1f%P --end-of-options <sha>…`, one process, exact given order. `--no-walk`: a plain lookup of exactly these commits, not a walk of history reachable from them. Resolves committer date (`%ct`, for the ordering below) and parent count (`%P`'s field count: ≥2 is a merge) for the whole selection in one call; fails the whole call if any sha doesn't resolve |
| Cherry-pick | `git cherry-pick --end-of-options <sha>…`. Git applies in argument order, so the app's oldest-first sort (FR-CP-02) is load-bearing and has its own tests. A merge commit anywhere in the selection is refused before this call ever runs (FR-CP-05): git's own per-commit refusal can leave earlier commits of the same batch already applied, so this app-level pre-check is what delivers "nothing changed". A stop with `CHERRY_PICK_HEAD` left behind (a real conflict, or an already-applied/empty pick — indistinguishable by exit code or state file alone) is classified `Code Conflicts`, left in place for the resolve flow; any other refusal (a dirty file in the way, another operation already running) passes through as plain `GitFailed`, git's own message |
| Revert | `git revert --no-edit --end-of-options <sha>…`, newest first (FR-RV-02, equally load-bearing). `--no-edit`: never waits on an interactive editor, regardless of whether the process happens to have a controlling terminal. Same merge pre-check and refusal classification as cherry-pick |
| Operation in progress | unmerged entries in status; `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge/`, `rebase-apply/`, `sequencer/` in the worktree's git dir. File checks, no subprocess. |
| Resolve / continue / abort | `git add -- <path>` (`StageFile`, unchanged) resolves every unmerged shape, a deletion included: staged with its current working-tree content for a path still present, or — the working file already removed — an explicit-pathspec `git add` on the now-missing path stages the removal and exits 0, the same as `git rm` would. `ContinueOperation`/`AbortOperation`: `git merge\|rebase\|cherry-pick\|revert --continue\|--abort`, `Continue: true` (→ `GIT_EDITOR=true`, TR-09: none of the four ever waits on a commit-message editor). `--continue` while any conflicted file remains is git's own refusal — exit 128 with the message on stderr for merge/cherry-pick/revert, exit 1 with the message on stdout for rebase — reclassified `Code Conflicts`, the same state the stop itself already reported. `--skip` is not offered. |
| Commit-graph (large repo, first open) | `git commit-graph write --reachable`, after the load's done signal, once `LoadLog` has loaded ≥ 20k rows and `<git-common-dir>/objects/info/commit-graph` doesn't exist yet | QA-PERF-02, ADR-4 exception: cuts `git log`'s time-to-first-record and peak memory at real scale; failure is logged, never surfaced; git keeps the file current itself during `gc`, nebegit never rewrites it |

### Update another branch in the background (FR-UPD)

Branch `B` with an upstream: `<remote>` and `<upstream-ref>` (the full ref on that remote, which may be named differently from `B`; `<remote>` is `.` for a local upstream), read in one `for-each-ref --format=%(objectname)%1f%(upstream:remotename)%1f%(upstream:remoteref)%1f%(worktreepath)%00 refs/heads/B` call. No upstream configured → `Code NoUpstream`, refused before any git call.

1. `B` not checked out in any worktree → `git fetch --progress <remote> <upstream-ref>:refs/heads/B`. Git allows this as a fast-forward only and refuses when `B` is checked out in any worktree (exit 128, `fatal: refusing to fetch into branch ...`), so the app's own worktree check is a pre-check, not the safety net. A non-fast-forward refusal exits 1 with an empty stdout and a stderr line ending in `(non-fast-forward)`; fetch has no `--porcelain` form the way push does, so this is stderr substring matching (`classifyDivergedFetch`), reclassifying the refusal `Code NotFastForward`. Any other refusal, including the checked-out-elsewhere race, stays plain `GitFailed`.
2. `B` checked out in another worktree → dirty check first (`Code DirtyWorktree` if not clean, no network spent on an already-refused call), then `git fetch --progress <remote> <upstream-ref>` (no destination refspec — only the shared remote-tracking ref moves, so the checked-out-branch refusal above never applies here), then, under that worktree's own write lock, `git -C <that worktree> merge --ff-only @{upstream}`.
3. `B` is this window's own current branch → Pull, not this feature; the UI never offers it here, but case 2's own mechanism still runs correctly if called this way (`B`'s own worktree is simply the one whose lock it takes).

Runs as a job, keyed `"update-branch:" + B` so two different branches' updates never dedupe into one (kind-based dedupe is otherwise per-repo). Result comes from comparing `B`'s SHA before and after (`rev-parse --verify --end-of-options`, `rev-list --count <old>..<new>`), read after the write: `Outcome` "updated" or "upToDate", `From`/`To`/`Commits` (FR-UPD-05).

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

- The common git dir is watched with fs notifications (`internal/watch.RepoWatcher`): `HEAD`, `refs/`, `packed-refs`, `index`, `worktrees/`, `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-*`, `sequencer/`. Per-worktree files live under `<git dir>/worktrees/<name>/`, so one watch covers all windows. `objects/`, `logs/`, `hooks/`, `info/` are not watched; a `*.lock` file is ignored (git's lockfile-then-rename pattern — the rename's target already classifies). fsnotify is primary; a 5 s low-frequency poll is an insurance fallback for what it misses (a dropped event, an exhausted watch count, a filesystem it doesn't support at all — e.g. WSL's 9p, decided per-platform). Debounce ~300 ms: an `onChange` fires once per distinct scope touched in the window, not once per fs event.
- What changed maps to one of four scopes, delivered as a `"repo:changed"` event's `scope`:

  | Base name | Scope |
  |---|---|
  | `HEAD`, `packed-refs`, any path under `refs/` (any depth) | `refs` |
  | `index` | `index` |
  | `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge`, `rebase-apply`, `sequencer` | `state` |
  | `worktrees/` itself, or a direct child of it | `worktrees` |

- Working files are not watched recursively. Status runs on window focus, after every own action, and on a timer while focused: start at 1 s so poll interval plus status time stays within 3 s; back off when status gets slow and show "refreshing". Polls are marked as background in the command log (FR-UX-02). A working-file poll only ever reports its own window's `index` scope (FR-REPO-04's working-file half); the fs-watched `.git/index` reports `index` repo-wide (the staging half).
- The app's own writes also trigger the watcher; debounce plus reload-after-write makes that harmless.
- The log/graph reloads only if its ref tips actually changed: a `refs` scope event re-checks the "Ref tips" row above against the snapshot recorded when the log last finished, and restarts the stream only on a difference — a cheap `for-each-ref`, not a re-walk.

### Jobs, progress, cancel

Mechanism delivered; no real job kind uses it yet.

- Every remote or slow action is a job with an id.
- Progress comes from git's `--progress` lines on stderr, always passed explicitly by the job-starting call: git suppresses progress reporting whenever stderr is not a terminal, which a subprocess of this app never is.
- `Runner.StreamErr` frames stderr on git's own convention: `\r` overwrites the line currently being drawn (a phase update), `\n` ends a completed one. `ParseProgressLine` is a best-effort parser over that framing — a percent and message where the line carries one, `ok=false` with just the trimmed, `remote:`-stripped message otherwise (a future git version wording a phase differently degrades to an unparsed status line, never a parse failure; this text carries no stability promise from git, unlike every other parser in `internal/git`).
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
