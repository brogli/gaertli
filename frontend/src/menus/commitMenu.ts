import type { Translate } from '../i18n'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

export type CommitMenuItem =
  { label: string; command: () => void } | { label: string; disabled: true } | { separator: true }

// PickActions are the cherry-pick and revert items the one-commit and range
// menus share.
interface PickActions {
  cherryPick: () => void
  cherryPickOptions: () => void
  revert: () => void
  revertOptions: () => void
}

interface OneCommitActions extends PickActions {
  openCreateBranch: (sha: string) => void
  openCheckoutCommit: (sha: string) => void
  openReset: (sha: string, subject: string) => void
  openCreateWorktree: (sha: string) => void
  compareWithWorking: () => void
  // compareWithPrevious is present only when another commit was already
  // selected before the right-click that opened this menu.
  compareWithPrevious?: { shortSha: string; run: () => void }
}

// SelectionActions is the menu for two or more selected commits, however
// they were selected: swap and stopComparing apply only with exactly two.
interface SelectionActions extends PickActions {
  copyHashes: () => void
  swap: () => void
  stopComparing: () => void
}

// buildOneCommitMenuItems is the menu for a plain right-click on a single
// commit.
export function buildOneCommitMenuItems(
  row: LogCommit,
  actions: OneCommitActions,
  t: Translate,
): CommitMenuItem[] {
  const items: CommitMenuItem[] = [
    {
      label: t('logView.menu.copyHash'),
      command: () => void navigator.clipboard.writeText(row.sha),
    },
    {
      label: t('logView.menu.copySubject'),
      command: () => void navigator.clipboard.writeText(row.subject),
    },
    { separator: true },
    {
      label: t('logView.menu.newBranchFromHere'),
      command: () => actions.openCreateBranch(row.sha),
    },
    { label: t('logView.menu.checkoutCommit'), command: () => actions.openCheckoutCommit(row.sha) },
    {
      label: t('logView.menu.resetToHere'),
      command: () => actions.openReset(row.sha, row.subject),
    },
    { separator: true },
    { label: t('logView.menu.cherryPick'), command: actions.cherryPick },
    { label: t('logView.menu.cherryPickOptions'), command: actions.cherryPickOptions },
    { label: t('logView.menu.revert'), command: actions.revert },
    { label: t('logView.menu.revertOptions'), command: actions.revertOptions },
    { separator: true },
    {
      label: t('logView.menu.createWorktreeHere'),
      command: () => actions.openCreateWorktree(row.sha),
    },
    { separator: true },
    { label: t('logView.menu.compareWithWorking'), command: actions.compareWithWorking },
  ]
  if (actions.compareWithPrevious) {
    const { shortSha, run } = actions.compareWithPrevious
    items.push({ label: t('logView.menu.compareWithPrevious', { sha: shortSha }), command: run })
  }
  return items
}

// buildSelectionMenuItems is the menu for a right-click inside a selection
// of two or more commits, however it was made. squash is the squash/fixup
// items already checked and built (buildSquashMenuItems), spliced in after
// cherry-pick and revert; empty while that check is still in flight.
export function buildSelectionMenuItems(
  rows: LogCommit[],
  actions: SelectionActions,
  squash: CommitMenuItem[],
  t: Translate,
): CommitMenuItem[] {
  const n = rows.length
  const items: CommitMenuItem[] = [
    { label: t('logView.menu.cherryPickCount', { n }, n), command: actions.cherryPick },
    {
      label: t('logView.menu.cherryPickCountOptions', { n }, n),
      command: actions.cherryPickOptions,
    },
    { label: t('logView.menu.revertCount', { n }, n), command: actions.revert },
    { label: t('logView.menu.revertCountOptions', { n }, n), command: actions.revertOptions },
  ]
  if (squash.length) items.push({ separator: true }, ...squash)
  items.push(
    { separator: true },
    { label: t('logView.menu.copyHashesCount', { n }, n), command: actions.copyHashes },
  )
  if (n === 2) {
    items.push(
      { separator: true },
      { label: t('logView.menu.swap'), command: actions.swap },
      { label: t('logView.menu.stopComparing'), command: actions.stopComparing },
    )
  }
  return items
}

interface SquashActions {
  squash: () => void
  fixup: () => void
}

// buildSquashMenuItems is the squash and fixup part of a menu on n
// selected commits, or, when blocker names why they cannot run, one
// disabled item saying so.
export function buildSquashMenuItems(
  n: number,
  blocker: string | null,
  actions: SquashActions,
  t: Translate,
): CommitMenuItem[] {
  if (blocker) {
    return [{ label: t('logView.menu.squashBlocked', { reason: blocker }), disabled: true }]
  }
  return [
    { label: t('logView.menu.squashCount', { n }, n), command: actions.squash },
    { label: t('logView.menu.fixupCount', { n }, n), command: actions.fixup },
  ]
}
