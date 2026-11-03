// Requirements: FR-CONF-05
import { describe, expect, it } from 'vitest'
import { i18n } from '@/i18n'
import { conflictSideTitles } from '../conflictSides'
import {
  Operation,
  type ConflictSide,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

const t = i18n.global.t

const side = (over: Partial<ConflictSide> = {}): ConflictSide => ({ present: true, ...over })
const branch = side({ ref: 'main', sha: '1234567890abcdef', subject: 'tip' })
const commit = side({ sha: 'abcdef0123456789', subject: 'Fix it' })

describe('conflictSideTitles', () => {
  it.each([
    [Operation.MergeOperation, 'main · current branch', 'abcdef0 Fix it · merged branch'],
    [Operation.RebaseOperation, 'main · rebased onto', 'abcdef0 Fix it · commit being replayed'],
    [Operation.CherryPickOperation, 'main · current branch', 'abcdef0 Fix it · picked commit'],
    [
      Operation.RevertOperation,
      'main · current branch',
      'abcdef0 Fix it · state before the reverted commit',
    ],
  ])('names the sides and roles of %s', (operation, current, incoming) => {
    expect(conflictSideTitles(operation, branch, commit, t)).toEqual({ current, incoming })
  })

  it('falls back to the short sha and subject when a side has no ref', () => {
    const titles = conflictSideTitles(Operation.MergeOperation, commit, commit, t)
    expect(titles.current).toBe('abcdef0 Fix it · current branch')
  })

  it('names a side without a sha by its role', () => {
    const titles = conflictSideTitles(Operation.$zero, side(), side(), t)
    expect(titles).toEqual({ current: 'Current side', incoming: 'Incoming side' })
  })

  it('names no-operation sides by ref and generic roles', () => {
    const titles = conflictSideTitles(Operation.$zero, branch, commit, t)
    expect(titles).toEqual({
      current: 'main · current side',
      incoming: 'abcdef0 Fix it · incoming side',
    })
  })
})
