# How features map to git

Git runs as `docs/conventions/git.md` describes. Command shapes below leave out the flags those
rules add everywhere.

## Commands

| Feature                   | Git                                                                                                                                                                                                                                                                                                                                        |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Open repo                 | `rev-parse --path-format=absolute --show-toplevel --git-dir --git-common-dir`                                                                                                                                                                                                                                                              |
| Summary                   | `symbolic-ref HEAD`, `rev-parse HEAD`; operation state from files in the git dir                                                                                                                                                                                                                                                           |
| Refs                      | `for-each-ref` over heads, remotes and tags, with upstream and ahead/behind; `remote`                                                                                                                                                                                                                                                      |
| Log                       | `log --topo-order --parents --decorate=full --exclude=refs/stash --all`, streamed (below)                                                                                                                                                                                                                                                  |
| Filters                   | the same `log` over the picked refs instead of `--all`, with `--author` (literal, case-insensitive) and `-- <path>`                                                                                                                                                                                                                        |
| Search                    | a hash resolves with `rev-parse`; text is `log --grep` (literal, case-insensitive) over the active filter, mapped onto loaded rows                                                                                                                                                                                                         |
| A commit's details        | `show --name-status -M --first-parent`                                                                                                                                                                                                                                                                                                     |
| Diff content              | both sides from a full-context `diff` (`show --first-parent --root` for a commit); hunks from a zero-context diff of the same pair, `-w` when ignoring whitespace; blobs via `cat-file --batch`                                                                                                                                            |
| Compare commits           | `diff --name-status -M <a> <b>`, so swapping mirrors the result                                                                                                                                                                                                                                                                            |
| Compare with working tree | `diff --name-status -M <sha>`                                                                                                                                                                                                                                                                                                              |
| Status                    | `status --porcelain=v2 --branch --untracked-files=all`                                                                                                                                                                                                                                                                                     |
| Stage, unstage            | `add`, `restore --staged`                                                                                                                                                                                                                                                                                                                  |
| Stage lines               | `apply --cached --recount` of a partial patch built from a fresh full-context diff (ADR-7)                                                                                                                                                                                                                                                 |
| Discard                   | `restore --worktree`, `clean -f` for untracked files, `apply --reverse` for lines                                                                                                                                                                                                                                                          |
| Commit, amend             | `commit [--amend] -F -`, the message on stdin; hooks run                                                                                                                                                                                                                                                                                   |
| Create branch             | `branch`, or `switch -c` to check it out; `--no-track` from a remote-tracking ref, so its first push sets an upstream of its own name                                                                                                                                                                                                      |
| Checkout                  | `switch`; `switch --track` for a remote branch; `switch --detach` for a commit                                                                                                                                                                                                                                                             |
| Reset                     | `reset --soft/--mixed/--hard/--keep`                                                                                                                                                                                                                                                                                                       |
| Rename, delete branch     | `branch -m`, `branch -d/-D`. Delete is refused before any git call for a branch another worktree has checked out, rebases or bisects. Rename is refused only for rebase or bisect; a branch checked out elsewhere is renamed there too, as git does                                                                                        |
| Cherry-pick, revert       | `cherry-pick [-n] [-x]`, `revert [-n] --no-edit`, several commits in one call, ordered by commit date, then topologically (FR-CP-02, FR-RV-02)                                                                                                                                                                                             |
| Continue, abort, skip     | `<operation> --continue/--abort/--skip`                                                                                                                                                                                                                                                                                                    |
| Squash, fixup             | an interactive rebase with a prepared todo (below)                                                                                                                                                                                                                                                                                         |
| Worktrees                 | `worktree list --porcelain`, `worktree add [-b --no-track]`, `worktree remove [--force]`, `worktree prune`                                                                                                                                                                                                                                 |
| Fetch                     | `fetch --prune`                                                                                                                                                                                                                                                                                                                            |
| Pull                      | `fetch`, then what `git pull` would run against `@{upstream}`: `rebase --fork-point`, or `merge --no-edit` with `pull.ff`'s fast-forward mode                                                                                                                                                                                              |
| Merge a branch            | `merge -s ort --no-edit --ff --commit --no-squash --no-autostash`: explicit, so `merge.*` config and a branch's `mergeOptions` cannot change the result                                                                                                                                                                                    |
| Rebase onto a branch      | `rebase --merge --no-rebase-merges --no-update-refs --no-autostash`: merges are flattened and no other branch moves, whatever `rebase.*` says                                                                                                                                                                                              |
| Stash and integrate       | `stash push`, then the pull, merge or rebase, then `stash pop` of exactly that entry, only after a clean integrate                                                                                                                                                                                                                         |
| Push                      | `push --porcelain`; its summary classifies rejections. Without an upstream: `push --porcelain --set-upstream <remote> refs/heads/<branch>`, naming the branch so a HEAD that moved meanwhile is not pushed; the remote from `%(push:remotename)`, else the only remote, else `origin`; `push.autoSetupRemote` decides whether to ask first |
| Force push                | `push --force-with-lease --force-if-includes`, never plain `--force`                                                                                                                                                                                                                                                                       |
| Commit-graph              | `commit-graph write --reachable`, in the background, after a log of 20,000 or more commits loads and none exists (QA-PERF-08)                                                                                                                                                                                                              |

Every remote or slow action (fetch, pull, push, force push, branch updates) is a job and passes
`--progress`, since git reports progress only to a terminal unless asked.

## Merge and rebase from the branch picker (FR-BR-11, FR-BR-12)

1. Refused before any git call while an operation is in progress or HEAD is detached.
2. The picked ref goes by its short name when that resolves to exactly it, so the merge message
   reads as on the command line ("Merge branch 'x'"); by its full name when a tag shares the name.
3. The result is classified by the state git leaves, not its exit code, which is 1 for a conflict, a
   refused fast-forward and a failed hook alike. An operation still in progress is a stop
   (`CONFLICTS` while files conflict, else `STOPPED`), left for continue or abort. A refusal over
   local changes offers the stash-and-retry Pull has (FR-SYNC-09).
4. Comparing HEAD before and after gives up to date, fast-forwarded, merged or rebased.

## Resolving a conflict (FR-CONF)

A conflicted file is read from what git left, never merged a second time.

1. `ls-files -u` gives its stages. Stage 2 is always the checked-out side and stage 3 the side
   brought in, in a rebase too. The current side's name comes from the operation's state files
   (`MERGE_HEAD`, `rebase-merge/onto`, `REBASE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`) and the
   refs pointing there.
2. The stages decide the kind: submodule, rename, deletion on one side, symlink; else the content. A
   NUL in the first 8000 bytes or a `merge` attribute of `binary` makes it binary, as for git; text
   that is not UTF-8 goes to the external editor.
3. For text, the result is the working file as git wrote it, in the user's conflict style, with any
   edits since; the sides are their blobs in working-tree form (`cat-file --filters`). Conflicts are
   its marker blocks (marker size from the `conflict-marker-size` attribute). Where each change sits
   in each side comes from git: the result with every block filled from one side is diffed against
   that side (`diff --no-index -U0`), and each side against the base the same way. With the blocks
   filled from one side, what still differs is a change only the other side made, which git applied.
   A change both sides made alike is listed nowhere. Starting every region from the base means
   taking the current side everywhere gives the current side's file, and taking every one-side
   change plus the current side in conflicts gives `merge -X ours`.
4. Text reaches the UI with LF line breaks. Saving writes the file's own line endings back, refuses
   when the file changed since it was read, and stages it only when no marker line is left.
5. Choosing a side is `checkout --ours`/`--theirs` and `add`; deleting is `rm`.
6. Reopening a resolved file restores its stages from the resolve-undo record `add` leaves
   (`ls-files --resolve-undo`, written back with `update-index --index-info`; `--unresolve` fails
   before git 2.43), then `checkout -m` rewrites the markers, or `--ours`/`--theirs` for a binary,
   symlink or one-side deletion. The record outlives the commit that concludes the operation, and
   `checkout -m` would then re-create a conflict on a clean tree, so reopening is offered only while
   the operation is stopped.

## Update another branch in the background (FR-UPD)

1. A branch `B` checked out nowhere: `fetch <remote> <upstream-ref>:refs/heads/B`. Git allows this
   only as a fast-forward and refuses it for a branch checked out in any worktree, so git is the
   safety net.
2. `B` checked out in a worktree, or being rebased or bisected there (git lists that worktree
   detached; the app reads its rebase and bisect state): refused before any git call, naming that
   worktree.

A diverged branch is `NOT_FAST_FORWARD`. Each update is its own job, so several run at once.

## Squash and fixup (FR-LOG-14)

Folding selected commits of the current branch is git's interactive rebase, run without an editor:

1. Checked first, read-only: the commits lie on one line from the oldest of them to HEAD with no
   merge on it (`merge-base --octopus`, `rev-list`); FR-LOG-17 lists the other blockers.
2. The core writes the whole todo: `pick` the oldest, `fixup` the others in order,
   `exec git commit --amend --no-verify --cleanup=verbatim -C <source>`, then `pick` the rest.
   `GIT_SEQUENCE_EDITOR` copies it over git's own using only shell builtins, so it works wherever
   git runs hooks.
3. The amend sets the message whatever `commit.cleanup` or the git version would make of a fixup
   chain, also after a stop and `--continue`, since it is part of the todo. Its source is the oldest
   commit for fixup; for squash, a `commit-tree` commit carrying the edited message and the oldest
   commit's author.
4. The rebase runs `--no-autostash` (a hidden stash would be a second thing to resolve on a
   conflict), `--no-update-refs` (other branches stay put), `--empty=keep` and
   `--reschedule-failed-exec`, with `rebase.missingCommitsCheck=error`, so a commit that joined the
   branch after the check stops the rebase instead of being dropped. A stop is left for continue or
   abort, as `CONFLICTS` or `STOPPED`.

Whether the branch then needs a force push is `merge-base --is-ancestor <oldest> @{upstream}`.

## Log loading and graph

1. One `git log` process; records are read as they stream and appended to an in-memory list, and the
   UI is told through `log:progress`, so the first rows show at once (FR-LOG-09, QA-PERF-02).
2. Lane assignment follows lazygit's streaming model: it needs only the previous row's lanes and the
   current commit. At most 32 lanes are drawn; a commit whose lane was hidden shows a "+k" marker
   where it is reached. A lane gets a colour number when it opens and a new one where it reaches a
   commit a local branch points at, decided per commit from the commit's own ref decorations, so no
   extra git call is needed and the stream stays a stream. A colour number is not reused while its
   lane stays open; the UI maps it onto its palette (FR-LOG-02).
3. A path filter keeps the graph valid, since `--parents` rewrites parents across filtered-out
   commits. An author filter shows a flat list. A new filter kills the running filtered log.
4. A `refs` change re-reads the ref tips and restarts the log only if they differ from the snapshot
   taken when it last finished.

## Noticing outside changes (FR-REPO-04)

The common git dir is watched with file-system notifications, with a slow poll for missed events and
file systems without notifications. Per-worktree files live under `<git dir>/worktrees/`, so one
watch covers a repo's worktrees. `*.lock` files are ignored, since git writes a lock and renames it
into place. Events are debounced and reported as one scope:

| Changed                                                                                      | Scope       |
| -------------------------------------------------------------------------------------------- | ----------- |
| `HEAD`, `packed-refs`, `refs/`, `config`                                                     | `refs`      |
| `index`                                                                                      | `index`     |
| `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REVERT_HEAD`, `rebase-merge`, `rebase-apply`, `sequencer` | `state`     |
| `worktrees/`                                                                                 | `worktrees` |

Working files are not watched. Status runs on window focus, after every own action, and on a timer
while focused: every second, doubling while status is slower than the interval, up to 10 s. A poll
compares the status output plus size and modification time of every listed file, since a further
edit to a listed file leaves the status output unchanged; a difference is scope `index`.

The app's own writes trigger the watcher too; debounce plus reload-after-write makes that harmless.

## Cancel

Cancel stops git as an interrupt in a terminal does. Every git call runs in its own process group;
the group (git, its hooks, its transport helpers) gets SIGTERM, on Windows a CTRL_BREAK event, and
git removes its locks and exits. What still runs after a grace period is killed; on Windows only git
itself, since reaching the rest would take a job object. A timeout stops git the same way.

The repo is left as git leaves it (QA-SAFE-04): an operation stopped mid-way shows in the operation
banner. The app never deletes a lock file: git's locks carry no owner, so a lock left by a killed
git is reported like any held lock (FR-UX-16).

## Auth

Git uses whatever the user has set up; with prompts off it fails fast. A sign-in failure is
`AUTH_FAILED` with git's text and the hint to sign in once from a terminal. SSH keys with a
passphrase and no agent fail; this is a documented limit.

## External editor (FR-ED)

- Every "open in editor" runs in the core (ADR-3), so for a WSL repo inside the distro. One profile
  serves every environment (FR-ED-11): WSL-aware editors (VS Code, Zed) start their Windows app from
  their WSL command.
- A profile is `{name, folderCmd, fileCmd, fileAtLineCmd?}` with `{path}` and `{line}` placeholders;
  `fileCmd` stands in for a missing `fileAtLineCmd`. The template is split into arguments first,
  then filled in. No shell.
- The editor starts detached, so it outlives the app; only start errors are reported
  (`EDITOR_NOT_FOUND`).
- No built-in editor: without a profile every open-in-editor reports that none is configured
  (FR-ED-05, FR-ED-06). Presets (FR-ED-07) are data. On a Windows host a preset opens each file in a
  window of its own where the editor would hand a lone file to its last window, which for a WSL
  remote window shows a Windows path as an empty file (FR-ED-13).
