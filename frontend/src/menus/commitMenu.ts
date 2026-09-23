import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// buildCommitMenuItems is the commit row's right-click menu. "Compare with
// selected" is absent rather than disabled when there is nothing to pair
// with. Items use OpenVue's {label, command} shape.
export function buildCommitMenuItems(
  row: LogCommit,
  actions: {
    openCreateBranch: (sha: string) => void
    openCheckoutCommit: (sha: string) => void
    openReset: (sha: string, subject: string) => void
    cherryPick: () => void
    cherryPickOptions: () => void
    revert: () => void
    revertOptions: () => void
    openCreateWorktree: (sha: string) => void
    compareWithSelected?: () => void
    compareWithWorking?: () => void
  },
): { label: string; command: () => void }[] {
  const items = [
    { label: 'Copy hash', command: () => void navigator.clipboard.writeText(row.sha) },
    { label: 'Copy subject', command: () => void navigator.clipboard.writeText(row.subject) },
    { label: 'New branch from here…', command: () => actions.openCreateBranch(row.sha) },
    { label: 'Check out commit', command: () => actions.openCheckoutCommit(row.sha) },
    { label: 'Reset current branch to here…', command: () => actions.openReset(row.sha, row.subject) },
    { label: 'Cherry-pick', command: () => actions.cherryPick() },
    { label: 'Cherry-pick…', command: () => actions.cherryPickOptions() },
    { label: 'Revert', command: () => actions.revert() },
    { label: 'Revert…', command: () => actions.revertOptions() },
    { label: 'Create worktree here…', command: () => actions.openCreateWorktree(row.sha) },
  ]
  if (actions.compareWithSelected) {
    items.push({ label: 'Compare with selected', command: actions.compareWithSelected })
  }
  if (actions.compareWithWorking) {
    items.push({ label: 'Compare with working tree', command: actions.compareWithWorking })
  }
  return items
}
