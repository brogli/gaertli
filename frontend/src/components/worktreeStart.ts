// WorktreeStart is a "Create worktree here…" prefill for
// CreateWorktreeDialog: a commit (always a new branch) or a branch (itself
// if local, a new branch's start point if remote). Its own module: a type
// exported from a <script setup> block resolves to `any` under eslint's
// type-aware linting.
export type WorktreeStart = { newBranch: true; start: string } | { newBranch: false; branch: string }
