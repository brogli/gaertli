import {
  DiffKind,
  type DiffTarget,
} from '../../bindings/github.com/brogli/gaertli/internal/app/models'
import type { ChangedFile } from '../../bindings/github.com/brogli/gaertli/internal/core'

// What a diff window shows. Imported from the generated models file, not
// the bindings barrel: unit tests and the e2e build replace the barrel.
export { DiffKind, type DiffTarget }

// fileTarget is target's diff of file: its path, and its old path for a
// rename.
export function fileTarget(
  file: ChangedFile,
  target: Omit<DiffTarget, 'path' | 'oldPath'>,
): DiffTarget {
  return { ...target, path: file.path, oldPath: file.oldPath ?? '' }
}

// readsWorkingTree tells whether target's diff reads the working tree, so
// an index-scope repo:changed can change it.
export function readsWorkingTree(target: DiffTarget): boolean {
  return target.kind === DiffKind.WorkingDiff || target.kind === DiffKind.AgainstWorkingDiff
}

// diffWindowQuery reads a diff window's worktree and target from its URL
// query, as the shell builds it; null for any other window.
export function diffWindowQuery(query: URLSearchParams): { wt: string; target: DiffTarget } | null {
  const wt = query.get('wt')
  const kind = Object.values(DiffKind).find((k) => k && k === query.get('kind'))
  if (query.get('view') !== 'diff' || !wt || !kind) return null
  return {
    wt,
    target: {
      kind,
      sha: query.get('sha') ?? '',
      from: query.get('from') ?? '',
      to: query.get('to') ?? '',
      staged: query.get('staged') === 'true',
      conflicted: query.get('conflicted') === 'true',
      path: query.get('file') ?? '',
      oldPath: query.get('oldFile') ?? '',
    },
  }
}
