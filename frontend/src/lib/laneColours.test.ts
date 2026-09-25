import { describe, expect, it } from 'vitest'
import { LANE_PALETTE_SIZE, laneColourClass } from './laneColours'

describe('laneColours', () => {
  it('FR-LOG-02 gives each colour under the palette size its own class', () => {
    expect(laneColourClass(0)).toBe('colour-0')
    expect(laneColourClass(LANE_PALETTE_SIZE - 1)).toBe(`colour-${LANE_PALETTE_SIZE - 1}`)
    expect(laneColourClass(1)).not.toBe(laneColourClass(2))
  })

  it('FR-LOG-02 repeats the palette once lanes outnumber it', () => {
    expect(laneColourClass(LANE_PALETTE_SIZE)).toBe(laneColourClass(0))
    expect(laneColourClass(LANE_PALETTE_SIZE + 2)).toBe(laneColourClass(2))
  })
})
