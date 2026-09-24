import type { Translate } from '../i18n'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

export type CommitMenuItem = { label: string; command: () => void } | { separator: true }

interface OneCommitActions {
  openCreateBranch: (sha: string) => void
  openCheckoutCommit: (sha: string) => void
  openReset: (sha: string, subject: string) => void
  cherryPick: () => void
  cherryPickOptions: () => void
  revert: () => void
  revertOptions: () => void
  openCreateWorktree: (sha: string) => void
  compareWithWorking: () => void
  // compareWithPrevious is present only when another commit was already
  // selected before the right-click that opened this menu.
  compareWithPrevious?: { shortSha: string; run: () => void }
}

interface RangeActions {
  cherryPick: () => void
  cherryPickOptions: () => void
  revert: () => void
  revertOptions: () => void
  copyHashes: () => void
}

interface ComparisonActions {
  swap: () => void
  stopComparing: () => void
  copyBothHashes: () => void
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

// buildRangeMenuItems is the menu for a right-click inside a Shift-click
// range of two or more commits.
export function buildRangeMenuItems(
  rows: LogCommit[],
  actions: RangeActions,
  t: Translate,
): CommitMenuItem[] {
  const n = rows.length
  return [
    { label: t('logView.menu.cherryPickCount', { n }, n), command: actions.cherryPick },
    {
      label: t('logView.menu.cherryPickCountOptions', { n }, n),
      command: actions.cherryPickOptions,
    },
    { label: t('logView.menu.revertCount', { n }, n), command: actions.revert },
    { label: t('logView.menu.revertCountOptions', { n }, n), command: actions.revertOptions },
    { separator: true },
    { label: t('logView.menu.copyHashesCount', { n }, n), command: actions.copyHashes },
  ]
}

// buildComparisonMenuItems is the menu for a right-click on one of the two
// commits already being compared.
export function buildComparisonMenuItems(
  actions: ComparisonActions,
  t: Translate,
): CommitMenuItem[] {
  return [
    { label: t('logView.menu.swap'), command: actions.swap },
    { label: t('logView.menu.stopComparing'), command: actions.stopComparing },
    { separator: true },
    { label: t('logView.menu.copyBothHashes'), command: actions.copyBothHashes },
  ]
}
