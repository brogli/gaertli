import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// buildCommitMenuItems is the commit row's right-click menu (FR-LOG-11's
// shell, spec 22): today it holds only "copy hash"/"copy subject" (moved
// from LogList's always-visible buttons, spec 10, FR-LOG-12). OpenVue's own
// item shape ({label, command}) is used directly — no separate own type for
// a two-item list (YAGNI). Later specs append their own items to this same
// builder: 21 (compare-selected), 40/41 (branch/reset/checkout-commit), 42
// (cherry-pick/revert), 50 (create worktree).
export function buildCommitMenuItems(row: LogCommit): { label: string; command: () => void }[] {
  return [
    { label: 'Copy hash', command: () => void navigator.clipboard.writeText(row.sha) },
    { label: 'Copy subject', command: () => void navigator.clipboard.writeText(row.subject) },
  ]
}
