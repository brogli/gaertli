# ADR-7: Git computes the diff, CodeMirror 6 merge draws it

Status: accepted

## Context

The diff view needs unified and side-by-side modes, change-by-change navigation, syntax
highlighting, ignore-whitespace, and hunk and line staging (FR-DIFF, FR-CHG-03). What it shows must
be what `git diff` shows, with the user's config deciding which lines changed (FR-DIFF-01,
QA-COMP-01), and a hunk action must act on exactly the lines the user sees. Two engines, one drawing
and one staging, align repeated lines differently.

## Decision

- **Git computes every diff.** Go returns both sides of a file in the form git diffed them, read
  from a full-context diff, and git's hunks from the same diff at zero context. Config that changes
  the edit itself stays in effect; config that only changes the patch's framing is overridden.
  `git-mapping.md` has the commands and the overrides.
- **CodeMirror draws it.** The view is `@codemirror/merge`, read-only, in its side-by-side and
  unified forms. Git's hunks replace CodeMirror's own diff, each as one whole-line change pinned at
  both edges, so CodeMirror cannot slide it onto a neighbouring equal line. Changed lines are marked
  whole.
- **Staging: Go owns the truth.** The UI sends the old- and new-side line numbers of the lines to
  act on, never an index into a hunk list, plus a fingerprint of both sides as shown. Go re-diffs
  the path fresh with the same config and refuses, writing nothing, when the sides differ from those
  shown. It builds a patch of only the selected lines and applies it with
  `git apply --cached --recount`; a discard reverse-applies it to the working tree. Only lines git
  reports changed are ever staged.
- **Line selections are completed per shown hunk**, as FR-CHG-03 and FR-CHG-10 describe, so a
  discard never drops a line the file had before.
- The whitespace-ignoring diff is for reading only (FR-DIFF-06): `git add -p` has no
  whitespace-ignoring mode, so hunk and line actions need the diff with whitespace shown.

Alternatives considered:

- CodeMirror computing the visual diff over raw bytes: it aligns repeated lines differently from
  git, ignores `diff.algorithm`, filters and line endings, and reimplements ignore-whitespace.
- An own diff renderer: every mode, highlighting and navigation to rebuild (ADR-8).

## Consequences

- The diff equals `git diff`, user config included, and one library covers every view mode.
- CodeMirror draws two of git's hunks fewer than three characters apart (a blank line or a lone
  brace between them) as one; its hunk action stages both and nothing else.
- The pins rely on the presentation rules of `@codemirror/merge`; a test runs git's hunks through
  the library itself, so an upgrade that changes those rules fails a test.
- The staging logic is tested against real git; its patch builder is adapted from lazygit (ADR-8).
- The merge editor (FR-CONF-05) draws git's conflict regions in plain CodeMirror editors and
  computes no diff of its own: the regions are the markers git wrote, and their places in each side
  come from git's diff. `@codemirror/merge` is not used there, since it pairs exactly two documents
  and its pinned hunks do not survive editing.
