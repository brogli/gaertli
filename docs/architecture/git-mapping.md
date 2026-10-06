# How features map to git

Every feature is a small number of git calls, made by the rules of TR-08 to TR-10: machine-readable
output only, a fixed environment, arguments as a list. Failures are classified as
`errors-security.md` describes. Beyond that:

- Every patch the app parses overrides the config that changes only its framing (colour, external
  diff tools, textconv, path prefixes). Config that changes the edit itself, such as the diff
  algorithm, attributes and filters, stays in effect (ADR-7).
- Nothing is reimplemented that git already does: checkout of a remote branch lets git derive the
  local branch and its upstream, git validates branch names, and pull is git's own fetch and merge
  or rebase.
- A ref the user picked reaches git by its full name (`refs/heads/x`, `refs/remotes/o/x`,
  `refs/tags/x`) wherever git reads it as a revision: a bare name that is both a branch and a tag is
  refused as a start point and read as the tag by `log`. Only a local branch to check out keeps its
  bare name: `switch` and `worktree add` would detach at a full ref.

| Feature                   | Git                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Open repo                 | `rev-parse --show-toplevel --git-dir --git-common-dir`                                                                                                                                                                                                                                                                                                                                                                                  |
| Summary                   | `symbolic-ref HEAD`, `rev-parse HEAD`; operation state from files in the git dir                                                                                                                                                                                                                                                                                                                                                        |
| Refs                      | `for-each-ref` over heads, remotes and tags, with upstream and ahead/behind                                                                                                                                                                                                                                                                                                                                                             |
| Log                       | `log --topo-order --parents --decorate=full --all`, streamed (below)                                                                                                                                                                                                                                                                                                                                                                    |
| Filters                   | the same `log` with `--author`, branches or tags and `-- <path>`                                                                                                                                                                                                                                                                                                                                                                        |
| Search                    | a hash resolves with `rev-parse`; otherwise `log --grep=<q>` over the active filter's revisions, author and path, mapped onto the loaded rows                                                                                                                                                                                                                                                                                           |
| A commit's details        | `show --name-status -M --first-parent`                                                                                                                                                                                                                                                                                                                                                                                                  |
| Diff content              | both sides from a full-context `diff` (`show --first-parent --root` for a commit); git's hunks from a zero-context diff of the same pair, with `-w` when whitespace is ignored; binary as git's attributes decide                                                                                                                                                                                                                       |
| Compare commits           | `diff --name-status -M <a> <b>`, the two-revision form, so swapping mirrors the result                                                                                                                                                                                                                                                                                                                                                  |
| Compare with working tree | `diff --name-status -M <sha>`                                                                                                                                                                                                                                                                                                                                                                                                           |
| Status                    | `status --porcelain=v2 --branch --untracked-files=all`                                                                                                                                                                                                                                                                                                                                                                                  |
| Stage, unstage            | `add`, `restore --staged`                                                                                                                                                                                                                                                                                                                                                                                                               |
| Stage lines               | `apply --cached --recount` of a partial patch built from a fresh full-context diff (ADR-7)                                                                                                                                                                                                                                                                                                                                              |
| Discard                   | `restore --worktree`, `clean -f` for untracked files, `apply --reverse` for lines                                                                                                                                                                                                                                                                                                                                                       |
| Commit, amend             | `commit [--amend] -F -`, the message on stdin; hooks run as usual                                                                                                                                                                                                                                                                                                                                                                       |
| Create branch             | `branch`, or `switch -c` when it is checked out at once; `--no-track` from a remote-tracking ref, so its first push gives it an upstream of its own name instead of the remote branch's                                                                                                                                                                                                                                                 |
| Checkout                  | `switch`; `switch --track <remote-tracking ref>` for a remote branch; `switch --detach` for a commit                                                                                                                                                                                                                                                                                                                                    |
| Reset                     | `reset --soft/--mixed/--hard/--keep`                                                                                                                                                                                                                                                                                                                                                                                                    |
| Rename, delete branch     | `branch -m`, `branch -d/-D`; delete refuses before any git call a branch another worktree has checked out, or a worktree is rebasing or bisecting (found as for Update, below), naming that worktree. Rename refuses the same way only for the rebase/bisect case, which git refuses anyway; for a branch checked out in another worktree it keeps git's own behaviour (git renames it there too and moves that worktree's HEAD along)  |
| Cherry-pick, revert       | `cherry-pick`, `revert`, several commits in one call, in the order FR-CP-02/FR-RV-02 require                                                                                                                                                                                                                                                                                                                                            |
| Continue, abort, skip     | `<operation> --continue/--abort/--skip`                                                                                                                                                                                                                                                                                                                                                                                                 |
| Squash, fixup             | `merge-base --octopus` and `rev-list` to find the commits' line, then `rebase -i` with a prepared todo (below)                                                                                                                                                                                                                                                                                                                          |
| Worktrees                 | `worktree list --porcelain`, `worktree add/remove/prune`; `add -b --no-track` from a remote-tracking ref, for the same reason as Create branch                                                                                                                                                                                                                                                                                          |
| Fetch                     | `fetch --prune`                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Pull                      | `fetch`, then the merge or rebase onto `@{upstream}` that `git pull --no-rebase`/`--rebase` would run, in the worktree queue (ADR-6)                                                                                                                                                                                                                                                                                                    |
| Merge a branch            | `merge --no-edit --ff --commit --no-squash --no-autostash` of the picked ref, in the worktree queue; explicit flags, so `merge.ff`, `merge.autoStash` and a branch's `mergeOptions` cannot change the result (below)                                                                                                                                                                                                                    |
| Rebase onto a branch      | `rebase --merge --no-rebase-merges --no-update-refs --no-autostash` onto the picked ref, in the worktree queue; merges are flattened and no other branch moves, whatever the `rebase.*` settings say                                                                                                                                                                                                                                    |
| Stash and integrate       | `stash push`, then the pull, merge or rebase, then `stash pop` of exactly that entry, only after a clean integrate                                                                                                                                                                                                                                                                                                                      |
| Push                      | `push --porcelain`; its summary classifies rejections. A branch without an upstream (`for-each-ref %(upstream)` of HEAD's ref is empty): `push --porcelain --set-upstream <remote> refs/heads/<branch>`, naming the branch so a HEAD that moved meanwhile is not pushed; the remote from `for-each-ref %(push:remotename)`, else the only remote, else `origin`; `config --type=bool push.autoSetupRemote` decides whether to ask first |
| Force push                | `push --force-with-lease --force-if-includes`, never plain `--force`                                                                                                                                                                                                                                                                                                                                                                    |
| Commit-graph              | `commit-graph write --reachable` once, on first open of a large repo (ADR-4)                                                                                                                                                                                                                                                                                                                                                            |

## Merge and rebase from the branch picker (FR-BR-11, FR-BR-12)

1. Refused before any git call while an operation is in progress or HEAD is detached.
2. The picked ref reaches git by its short name when that name resolves to exactly that ref
   (`rev-parse --verify --symbolic-full-name`), so the merge commit's default message reads as on
   the command line ("Merge branch 'x'"); when a tag shares the name, by its full ref.
3. A failure is classified by the state git leaves, not by its exit code, which is 1 for a conflict,
   a refused fast-forward and a failed hook alike: an operation still in progress is a stop
   (`CONFLICTS` while files conflict, else `STOPPED`), left for continue or abort; a refusal over
   local changes offers the stash-and-retry Pull has (FR-SYNC-09).
4. The result compares HEAD before and after: up to date, fast-forwarded, merged or rebased.

## Resolving a conflict (FR-CONF)

A conflicted file is read from what git left, never merged a second time:

1. `ls-files -u` gives its stages. Stage 2 is always the side checked out and stage 3 the side
   brought in, in a rebase too, where the checked-out side is the branch rebased onto. So the
   current side is stage 2 (`--ours`) for every operation; only its name changes with the operation,
   read from the operation's state files (`MERGE_HEAD`, `rebase-merge/onto`, `REBASE_HEAD`,
   `CHERRY_PICK_HEAD`, `REVERT_HEAD`) and the refs pointing there.
2. The stages decide the kind: a submodule, a rename (one stage only), a deletion on one side, a
   symlink; otherwise the contents do. A NUL in the first 8000 bytes or a `merge` attribute of
   `binary` makes it binary, as for git; text that is not UTF-8 goes to the external editor.
3. For text, the result is the working file as git wrote it, in the user's conflict style, with any
   edits made since; the sides are their blobs in working-tree form (`cat-file --filters`). The
   conflict regions are its marker blocks (marker size from the `conflict-marker-size` attribute).
   Where each region sits in each side comes from git: the result with every block filled from one
   side is diffed against that side (`diff --no-index -U0`).
4. All text reaches the UI with LF line breaks. Saving writes the working file's own convention
   back, refuses when the file changed since it was read, and stages it (`add`) only when no marker
   line is left.
5. Choosing a side is `checkout --ours`/`--theirs` and `add`; deleting is `rm`.
6. Reopening a resolved file uses the resolve-undo record `add` leaves: `update-index --unresolve`,
   then `checkout -m` for a file with both sides, which writes the merge with its markers again. The
   record outlives the commit that concludes the operation, and `checkout -m` would then re-create a
   conflict on a clean tree, so reopening is offered and run only while the operation is still
   stopped (`ls-files --resolve-undo`).

## Update another branch in the background (FR-UPD)

Updating a branch `B` from its upstream without checking it out:

1. `B` is checked out nowhere: `git fetch <remote> <upstream-ref>:refs/heads/B`. Git allows this
   only as a fast-forward and refuses it for a branch checked out in any worktree, so git itself is
   the safety net.
2. `B` is checked out in a worktree, this window's included, or a worktree is rebasing or bisecting
   it (git lists that worktree detached; the app reads its rebase and bisect state to find the
   branch): refused before any git call, naming that worktree. Bringing it up to date is a pull
   there, or finishing the rebase or bisect first.

A diverged branch is reported as `NOT_FAST_FORWARD`. Each update is its own job, so several branches
can update at once. The result compares `refs/heads/B` before and after: updated by _n_ commits, or
up to date.

## Squash and fixup (FR-LOG-14)

Folding selected commits of the current branch is git's own interactive rebase, run without an
editor:

1. The selection is checked first, read-only: the selected commits must lie on one line from the
   oldest of them to HEAD, with no merge on it; FR-LOG-14 lists the other blockers. There is no
   autostash: a hidden stash entry would be a second thing to resolve when a move conflicts.
2. The core writes the whole todo: `pick` the oldest selected commit, `fixup` the others in their
   order, `exec git commit --amend --cleanup=verbatim -C <source>`, then `pick` the rest.
   `GIT_SEQUENCE_EDITOR` copies that todo over git's own, using only builtins of the shell git runs
   hooks in, so it works on every platform.
3. The amend sets the message whatever `commit.cleanup` or the git version would make of a fixup
   chain, and it runs after a stop and `--continue` too, since it is part of the todo. Its source is
   the oldest commit for fixup; for squash, a commit made with `commit-tree` that carries the edited
   message and the oldest commit's author.
4. The rebase runs in the worktree queue. It overrides the user's autostash setting (step 1) and
   update-refs setting, so other branches stay where they are; a commit that joined the branch after
   the check stops the rebase instead of being dropped. A rebase that stops is left for continue or
   abort, reported as `CONFLICTS` or `STOPPED`.

Whether the branch then needs a force push is `merge-base --is-ancestor <oldest> @{upstream}`.

## Log loading and graph

1. One `git log` process; records are read as they stream.
2. Lane assignment follows lazygit's streaming model, ported into `internal/graph`. It needs only
   the previous row's lanes and the current commit. Lanes are capped; rows beyond the cap show a
   "+k" marker. Each lane gets a colour number when it opens, not reused while it stays open; the UI
   maps that number onto its own small palette (FR-LOG-02).
3. Rows are appended to an in-memory list and the UI is told through `log:progress`, so the first
   rows show at once (FR-LOG-09, QA-PERF-02). A failed load ends with a progress event carrying
   git's error.
4. The UI requests row windows by index.
5. A path filter keeps the graph valid, because `--parents` rewrites parents across filtered-out
   commits. An author filter shows a flat list without graph lines. A search only highlights rows of
   the loaded view.
6. A new filter kills the running filtered `git log` and starts a new one.
7. A `refs` change (below) re-reads the ref tips and restarts the log only if they differ from the
   snapshot taken when the log last finished.

## Noticing outside changes (FR-REPO-04)

The common git dir is watched with file-system notifications, backed by a slow poll for missed
events and file systems without notifications. Per-worktree files live under
`<git dir>/worktrees/<name>/`, so one watch covers all worktrees of a repo. Lock files are ignored,
because git writes a lock file and renames it into place. Events are debounced and reported as one
of four scopes:

| Changed                                                                                                          | Scope       |
| ---------------------------------------------------------------------------------------------------------------- | ----------- |
| `HEAD`, `packed-refs`, anything under `refs/`, `config` (remotes, upstreams)                                     | `refs`      |
| `index`                                                                                                          | `index`     |
| operation state: `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge/`, `rebase-apply/`, `sequencer/` | `state`     |
| `worktrees/` or a direct child                                                                                   | `worktrees` |

Working files are not watched recursively. Instead, status runs on window focus, after every own
action, and on a timer while the window is focused. The timer starts at 1 s, so an outside edit
shows within 3 s, and backs off when status gets slow. A poll compares the status output plus the
size and modification time of every changed file, because a further edit to a file already listed
leaves the status output unchanged; a difference is reported as scope `index`.

The app's own writes also trigger the watcher. Debounce plus reload-after-write makes that harmless.

## Jobs, progress, cancel

- Every remote or slow action is a job with an id: fetch, pull, push, force push, branch updates.
- Progress comes from git's `--progress` output on stderr. Git reports progress only to a terminal
  unless asked, so every job passes `--progress`. Parsing it is best effort: unknown wording shows
  as a plain status line.
- Cancel stops git the way an interrupt in a terminal does. Every git call runs in a process group
  of its own; the group (git, its hooks, its transport helpers) gets SIGTERM, on Windows a
  CTRL_BREAK event, and git removes its lock files and exits. What still runs after a grace period
  is killed; on Windows only git itself, since reaching the rest of the group would take a job
  object. A call's timeout stops git the same way.
- The repo is left as git leaves it when interrupted (QA-SAFE-04): an operation git stopped mid-way
  (a pull's rebase) shows in the operation banner for continue or abort. The app never deletes a
  lock file: git's locks carry no owner, so a lock left by a git that had to be killed is reported
  like any other held lock (FR-UX-16).

## Auth

- Git uses whatever the user has set up. With terminal prompts off (TR-09) it fails fast instead of
  waiting for input.
- A sign-in failure is shown as `AUTH_FAILED` with git's text and the hint to log in once from a
  terminal.
- SSH keys with a passphrase and no agent fail. This is a documented limit.

## External editor (FR-ED)

- Every "open in editor" action runs in the core (ADR-3), so profiles are stored per environment
  (FR-ED-11, TR-13).
- A profile is `{name, folderCmd, fileCmd, fileAtLineCmd?}` with the placeholders `{path}` and
  `{line}`. Without `fileAtLineCmd`, `fileCmd` is used.
- The template is split into arguments first, then the placeholders are filled in. No shell (TR-10).
- The editor starts detached (TR-12); only start errors are reported, as `EDITOR_NOT_FOUND` naming
  the command.
- Presets (FR-ED-07) are data in Settings.
