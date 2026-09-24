# ADR-7: Git computes the diff, CodeMirror 6 merge draws it

Status: accepted

## Context

The diff view needs unified and side-by-side modes, change-by-change navigation, syntax highlighting, ignore-whitespace, and hunk and line staging (FR-DIFF, FR-CHG-03). What it shows must be what `git diff` shows, with the user's config deciding which lines changed (FR-DIFF-01, QA-COMP-01), and a hunk action must act on exactly the lines the user sees. Two engines, one drawing and one staging, align repeated lines differently.

## Decision

- **Git computes every diff.** Go returns both sides of a file in the form git diffed them, read from a full-context `git diff` (or `git show` for a commit), and git's hunks from the same diff with `-U0 --inter-hunk-context=0`, plus `-w` when whitespace is ignored. Config that changes the edit itself stays in effect; config that only changes the patch's framing is overridden (`git-mapping.md` lists it).
- **CodeMirror draws it.** The view is `@codemirror/merge`, read-only: `MergeView` side by side, `unifiedMergeView` unified. Git's hunks reach it through `DiffConfig.override`, each as one whole-line change with a zero-length change pinned at each edge, so CodeMirror's presentation pass cannot slide it. Changed lines are marked whole.
- **Staging: Go owns the truth.** The UI sends the old- and new-side line numbers of the lines to act on, never an index into a hunk list, plus a fingerprint of both sides as shown. Go re-diffs the path fresh with the same config and refuses, writing nothing, when the sides differ from those shown. It keeps only the selected lines, writes each partial hunk where the hunks before it leave the file, and applies the patch with `git apply --cached --recount`; a discard reverse-applies it to the working tree. Only lines git reports changed are ever staged, and a file's mode change is never part of a hunk.
- **Line selections are completed per shown hunk**, as FR-CHG-03 and FR-CHG-10 describe, so a discard never drops a line the file had before.
- The whitespace-ignoring diff is for reading only (FR-DIFF-06).

Alternatives considered:

- CodeMirror computing the visual diff over raw bytes: it aligns repeated lines differently from git, ignores `diff.algorithm`, filters and line endings, and reimplements ignore-whitespace.
- An own diff renderer: every mode, highlighting and navigation to rebuild (ADR-8).

## Consequences

- The diff equals `git diff`, user config included, and one library covers every view mode.
- CodeMirror joins two of git's hunks fewer than three characters apart (a blank line or a lone brace between them) into one chunk; that chunk's hunk action stages both hunks and nothing else.
- The pins rely on the presentation rules of `@codemirror/merge`; a test runs git's hunks through its real chunk builder, so an upgrade that changes them fails a test.
- The editors split lines at `\n` only, as git does, so a lone carriage return stays inside its line.
- The staging logic lives in Go and is tested against real git. The patch builder is adapted from lazygit (ADR-8).
