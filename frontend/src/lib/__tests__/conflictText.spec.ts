import { describe, expect, it } from 'vitest'
import { conflictFileFixture } from '@/testing/fixtures/conflicts'
import { resultFromBase, takenLines } from '../conflictText'

describe('conflictText', () => {
  it('FR-CONF-05 the result holds the base lines in place of each marker block', () => {
    const file = conflictFileFixture()

    expect(resultFromBase(file)).toEqual({
      text: 'a\no1\nb\no2\nz',
      regions: [
        { start: 2, lines: 1 },
        { start: 4, lines: 1 },
      ],
    })
  })

  it('FR-CONF-05 a conflict without a base leaves its regions empty', () => {
    const file = conflictFileFixture({ base: { present: false } })
    const regions = file.regions!.map((region) => ({ ...region, base: { start: 0, lines: 0 } }))

    expect(resultFromBase({ ...file, regions })).toEqual({
      text: 'a\nb\nz',
      regions: [
        { start: 1, lines: 0 },
        { start: 2, lines: 0 },
      ],
    })
  })

  it('FR-CONF-05 builds each take from the side lines', () => {
    const file = conflictFileFixture()

    expect(takenLines(file, 1, 'current')).toEqual(['c2'])
    expect(takenLines(file, 1, 'incoming')).toEqual(['i2'])
    expect(takenLines(file, 1, 'currentFirst')).toEqual(['c2', 'i2'])
    expect(takenLines(file, 0, 'incomingFirst')).toEqual(['i1', 'c1'])
  })
})
