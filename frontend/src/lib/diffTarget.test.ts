import { describe, expect, it } from 'vitest'
import { DiffKind, diffWindowQuery } from './diffTarget'

const query = (search: string) => new URLSearchParams(search)

describe('diffWindowQuery', () => {
  it.each([
    [
      'view=diff&wt=/repo&kind=commit&sha=abc&file=a.txt',
      {
        kind: DiffKind.CommitDiff,
        sha: 'abc',
        from: '',
        to: '',
        staged: false,
        path: 'a.txt',
        oldPath: '',
      },
    ],
    [
      'view=diff&wt=/repo&kind=range&from=x&to=y&file=new.txt&oldFile=old.txt',
      {
        kind: DiffKind.RangeDiff,
        sha: '',
        from: 'x',
        to: 'y',
        staged: false,
        path: 'new.txt',
        oldPath: 'old.txt',
      },
    ],
    [
      'view=diff&wt=/repo&kind=working&staged=true&file=a.txt',
      {
        kind: DiffKind.WorkingDiff,
        sha: '',
        from: '',
        to: '',
        staged: true,
        path: 'a.txt',
        oldPath: '',
      },
    ],
  ])(
    'FR-DIFF-07 reads the worktree and the target from the URL the shell builds: %s',
    (search, target) => {
      expect(diffWindowQuery(query(search))).toEqual({ wt: '/repo', target })
    },
  )

  it.each(['path=/repo', 'view=diff&kind=commit&sha=abc', 'view=diff&wt=/repo&kind=bogus'])(
    'FR-DIFF-07 %s is no diff window',
    (search) => {
      expect(diffWindowQuery(query(search))).toBeNull()
    },
  )
})
