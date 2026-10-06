import type { Translate } from '@/i18n'
import { shortSha } from '@/lib/format'
import {
  Operation,
  type ConflictSide,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

interface Roles {
  current: string
  incoming: string
}

const ROLES: Partial<Record<Operation, Roles>> = {
  [Operation.MergeOperation]: { current: 'currentBranch', incoming: 'mergedBranch' },
  [Operation.RebaseOperation]: { current: 'rebasedOnto', incoming: 'commitBeingReplayed' },
  [Operation.CherryPickOperation]: { current: 'currentBranch', incoming: 'pickedCommit' },
  [Operation.RevertOperation]: { current: 'currentBranch', incoming: 'stateBeforeRevertedCommit' },
}
const NO_OPERATION: Roles = { current: 'currentSide', incoming: 'incomingSide' }

// sideName is a side's ref, else its short sha and subject; undefined when
// the side has no commit.
function sideName(side: ConflictSide, useRef = true): string | undefined {
  if (!side.sha) return undefined
  const commit = `${shortSha(side.sha)} ${side.subject ?? ''}`.trim()
  return (useRef && side.ref) || commit
}

// incomingName is the incoming side's name; a revert's is only its commit.
function incomingName(operation: Operation, incoming: ConflictSide): string | undefined {
  return sideName(incoming, operation !== Operation.RevertOperation)
}

// paneTitle is the side's name and role, or the role alone for a side
// without a commit.
function paneTitle(name: string | undefined, role: string, t: Translate): string {
  return name
    ? t('mergeEditor.paneHeader', { name, role })
    : role.charAt(0).toUpperCase() + role.slice(1)
}

// conflictSideNames names a conflict's two sides without their roles:
// "main".
export function conflictSideNames(
  operation: Operation,
  current: ConflictSide,
  incoming: ConflictSide,
  t: Translate,
): Roles {
  return {
    current: sideName(current) ?? t('mergeEditor.currentSide'),
    incoming: incomingName(operation, incoming) ?? t('mergeEditor.incomingSide'),
  }
}

// conflictSideTitles names a conflict's two sides and their roles for the
// operation, as a pane or panel title: "main · current branch".
export function conflictSideTitles(
  operation: Operation,
  current: ConflictSide,
  incoming: ConflictSide,
  t: Translate,
): Roles {
  const roles = ROLES[operation] ?? NO_OPERATION
  const role = (key: string) => t(`mergeEditor.role.${key}`)
  return {
    current: paneTitle(sideName(current), role(roles.current), t),
    incoming: paneTitle(incomingName(operation, incoming), role(roles.incoming), t),
  }
}
