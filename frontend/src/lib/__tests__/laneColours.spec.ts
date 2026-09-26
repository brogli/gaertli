import { describe, expect, it } from 'vitest'
import { LANE_PALETTE_SIZE, laneColourClass } from '../laneColours'

describe('laneColours', () => {
  it('FR-LOG-02 gives each colour under the palette size its own class', () => {
    const palette = Array.from({ length: LANE_PALETTE_SIZE }, (_, i) => laneColourClass(i))
    expect(new Set(palette).size).toBe(LANE_PALETTE_SIZE)
  })

  it('FR-LOG-02 repeats the palette once lanes outnumber it', () => {
    expect(laneColourClass(LANE_PALETTE_SIZE)).toBe(laneColourClass(0))
    expect(laneColourClass(LANE_PALETTE_SIZE + 2)).toBe(laneColourClass(2))
  })
})
