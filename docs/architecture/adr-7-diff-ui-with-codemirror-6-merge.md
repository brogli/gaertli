# ADR-7: Diff UI with CodeMirror 6 merge

Status: accepted

## Context

The diff view needs unified and side-by-side modes, collapsed unchanged regions, change-by-change navigation, syntax highlighting, and hunk and line staging (FR-DIFF, FR-CHG-03).

## Decision

- The view is `@codemirror/merge`: `MergeView` for side-by-side, `unifiedMergeView` for unified, both read-only, with `collapseUnchanged` and chunk navigation.
- Go delivers the old and new content of a file (`cat-file --batch`, or the working-tree file). CodeMirror computes the visual diff.
- For hunk and line staging, Go owns the truth. The UI sends the old- and new-side line numbers of the lines to stage, never an index into a hunk list. Go re-diffs the path fresh, keeps only the selected lines, and applies the resulting partial patch with `git apply --cached`. Because lines are addressed by number, the call does not depend on how either engine groups changes into hunks. Git merges changes within a few lines of each other into one hunk; CodeMirror does not.

## Consequences

- One library covers every diff mode, with highlighting and navigation included.
- The staging logic lives in Go and is tested against real git.
- The patch builder is our own code, after lazygit's pattern (ADR-8).
- The displayed diff is CodeMirror's, not git's. On files with repeated lines the two can align changes differently, so a hunk action acts only on the lines both agree changed. The displayed content also ignores the user's `diff.algorithm`, clean/smudge filters and line-ending attributes.
