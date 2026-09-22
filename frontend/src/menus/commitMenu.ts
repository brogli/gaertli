import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// buildCommitMenuItems is the commit row's right-click menu (FR-LOG-11's
// shell, spec 22): "copy hash"/"copy subject" (moved from LogList's
// always-visible buttons, spec 10, FR-LOG-12), plus "new branch from
// here…"/"check out commit" (spec 40, FR-BR-01/FR-BR-08), "reset current
// branch to here…" (spec 41, FR-BR-02) and "cherry-pick"/"revert" (spec 42,
// FR-CP-01/FR-RV-01: act on the log's multi-select if the right-clicked row
// is part of it, else on that row alone — LogList.vue's own decision, not
// this builder's). OpenVue's own item shape ({label, command}) is used
// directly — no separate own type for a short list (YAGNI). actions opens
// the dialogs the view itself owns (one instance each, not one per
// virtualized row — WorktreeView.vue's own decision), or runs a stateless
// write action directly (cherryPick/revert). compareWithSelected (spec 23,
// FR-CMP-01/FR-LOG-11) is appended last, only when provided — LogList.vue's
// own decision: absent rather than disabled when there is no previously
// selected commit to pair row with. Later specs append their own items to
// this same builder: 50 (create worktree).
export function buildCommitMenuItems(
  row: LogCommit,
  actions: {
    openCreateBranch: (sha: string) => void
    openCheckoutCommit: (sha: string) => void
    openReset: (sha: string, subject: string) => void
    cherryPick: () => void
    revert: () => void
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
  ]
  if (actions.compareWithSelected) {
    items.push({ label: 'Compare with selected', command: actions.compareWithSelected })
  }
  return items
}
