# ADR-7: Diff UI with CodeMirror 6 merge

- `@codemirror/merge`: `MergeView` (side-by-side) and `unifiedMergeView` (unified), `collapseUnchanged`, chunk navigation, language highlighting, read-only.
- Go delivers old and new file content (`git show <rev>:<path>`, `cat-file --batch`); CodeMirror computes the visual diff.
- For hunk and line staging Go owns the truth: it takes `git diff` hunks, builds a partial patch, applies it to the index. The UI sends "hunk N" or "lines a–b of hunk N".
- Spike (M6): mapping CodeMirror's chunks to git's hunks. If messy, the changes view renders git's own hunks.

