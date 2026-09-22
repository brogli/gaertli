import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// buildCommitMenuItems is the commit row's right-click menu (FR-LOG-11):
// copy hash/subject (FR-LOG-12), new branch/check out commit (FR-BR-01,
// FR-BR-08), reset (FR-BR-02), cherry-pick/revert (FR-CP-01, FR-RV-01),
// create worktree here (FR-WT-06) and, when provided, compare with the
// previously selected commit (FR-CMP-01) — absent rather than disabled when
// there is nothing to pair with. Items use OpenVue's own {label, command}
// shape. actions opens the dialogs the view owns, or runs a stateless write
// action directly.
export function buildCommitMenuItems(
  row: LogCommit,
  actions: {
    openCreateBranch: (sha: string) => void
    openCheckoutCommit: (sha: string) => void
    openReset: (sha: string, subject: string) => void
    cherryPick: () => void
    revert: () => void
    openCreateWorktree: (sha: string) => void
    compareWithSelected?: () => void
  },
): { label: string; command: () => void }[] {
  const items = [
    { label: 'Copy hash', command: () => void navigator.clipboard.writeText(row.sha) },
    { label: 'Copy subject', command: () => void navigator.clipboard.writeText(row.subject) },
    { label: 'New branch from here…', command: () => actions.openCreateBranch(row.sha) },
    { label: 'Check out commit', command: () => actions.openCheckoutCommit(row.sha) },
    { label: 'Reset current branch to here…', command: () => actions.openReset(row.sha, row.subject) },
    { label: 'Cherry-pick', command: () => actions.cherryPick() },
    { label: 'Revert', command: () => actions.revert() },
    { label: 'Create worktree here…', command: () => actions.openCreateWorktree(row.sha) },
  ]
  if (actions.compareWithSelected) {
    items.push({ label: 'Compare with selected', command: actions.compareWithSelected })
  }
  return items
}
