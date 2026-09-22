// WorktreeStart is a "create worktree here…" entry point's own prefill for
// CreateWorktreeDialog (FR-WT-06): a commit (LogList's context
// menu, always newBranch) or a branch (RefsSidebar's context menu — the
// branch itself for a local one, its name as a new branch's start point for
// a remote one). A plain module, not exported from CreateWorktreeDialog.vue
// itself: a type-only export from a <script setup> block resolves to `any`
// under eslint's type-aware linting (typescript-eslint/no-redundant-type-
// constituents), even though vue-tsc itself accepts it.
export type WorktreeStart = { newBranch: true; start: string } | { newBranch: false; branch: string }
