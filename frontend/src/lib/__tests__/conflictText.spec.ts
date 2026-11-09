// Requirements: FR-CONF-05, FR-CONF-10
import { describe, expect, it } from 'vitest'
import { changesFileFixture, conflictFileFixture } from '@/testing/fixtures/conflicts'
import { resultFromBase, takenLines } from '../conflictText'

describe('conflictText', () => {
  it('the result holds the base lines in place of each marker block', () => {
    const file = conflictFileFixture()

    expect(resultFromBase(file)).toEqual({
      text: 'a\no1\nb\no2\nz',
      changes: [
        { start: 2, lines: 1 },
        { start: 4, lines: 1 },
      ],
    })
  })

  it('a conflict without a base leaves its changes empty', () => {
    const file = conflictFileFixture({ base: { present: false } })
    const changes = file.changes!.map((change) => ({ ...change, base: { start: 0, lines: 0 } }))

    expect(resultFromBase({ ...file, changes })).toEqual({
      text: 'a\nb\nz',
      changes: [
        { start: 1, lines: 0 },
        { start: 2, lines: 0 },
      ],
    })
  })

  it('the result holds the base lines of every change, also one with no lines in the result', () => {
    const file = changesFileFixture()
    const deleted = {
      ...file.changes![0]!,
      result: { start: 1, lines: 0 },
      base: { start: 2, lines: 1 },
    }
    const state = {
      ...file,
      base: { present: true, content: 'x\nold\ny' },
      result: 'x\ny',
      changes: [deleted],
    }

    expect(resultFromBase(state)).toEqual({
      text: 'x\nold\ny',
      changes: [{ start: 2, lines: 1 }],
    })
    expect(resultFromBase({ ...file, changes: file.changes!.slice(0, 1) }).text).toBe(
      'a\no1\nb\n<<<<<<< HEAD\nc2\n=======\ni2\n>>>>>>> side\nc\ni3\nz\nnew',
    )
  })

  it('builds each take from the side lines', () => {
    const file = conflictFileFixture()

    expect(takenLines(file, 1, 'current')).toEqual(['c2'])
    expect(takenLines(file, 1, 'incoming')).toEqual(['i2'])
    expect(takenLines(file, 1, 'currentFirst')).toEqual(['c2', 'i2'])
    expect(takenLines(file, 0, 'incomingFirst')).toEqual(['i1', 'c1'])
  })
})
