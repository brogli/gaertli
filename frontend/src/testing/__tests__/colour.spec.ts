import { describe, expect, it } from 'vitest'
import { contrastRatio, deltaEOK, parseColor } from '../colour'

describe('colour', () => {
  it('FR-DIFF-11 contrastRatio of white against black is 21:1', () => {
    expect(contrastRatio('#ffffff', '#000000')).toBeCloseTo(21, 1)
  })

  it('FR-DIFF-11 deltaEOK of white against black is 1.0', () => {
    expect(deltaEOK('#ffffff', '#000000')).toBeCloseTo(1, 2)
  })

  it('FR-DIFF-11 parseColor reads color(srgb 1 1 1) as white', () => {
    expect(parseColor('color(srgb 1 1 1)')).toEqual({ r: 255, g: 255, b: 255 })
  })
})
