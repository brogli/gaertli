# ADR-7: Git computes the diff, CodeMirror draws it

**Status.** Approved.

**Context.** The diff view needs side-by-side and unified modes, highlighting and line staging
(FR-DIFF, FR-CHG-03). It must show what `git diff` shows, under the user's config (QA-COMP-01), and
a hunk action must act on exactly the lines shown. Two diff engines, one drawing and one staging,
align repeated lines differently.

**Decision.** Git computes every diff; CodeMirror only draws it, with git's hunks pinned in place of
its own. Staging belongs to Go: the UI names the lines, Go re-diffs, refuses if the file changed
since it was shown, and applies a patch of just those lines. The merge editor likewise draws the
changes git produced and computes no diff of its own.

Rejected: CodeMirror's own diff (ignores git's algorithm, filters and line endings) and an own
renderer (every mode to rebuild).

**Consequences.** The diff equals `git diff`, and one library covers every mode. The pinning depends
on CodeMirror's presentation rules, so a test runs git's hunks through the library to catch an
upgrade that changes them.
