import {
  DiffKind,
  type DiffTarget,
} from '../../bindings/github.com/brogli/nebegit/internal/app/models'

// What a diff window shows. Imported from the generated models file, not
// the bindings barrel: unit tests and the e2e build replace the barrel.
export { DiffKind, type DiffTarget }

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
      path: query.get('file') ?? '',
      oldPath: query.get('oldFile') ?? '',
    },
  }
}
