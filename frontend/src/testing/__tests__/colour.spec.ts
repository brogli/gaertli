import { describe, expect, it } from 'vitest'
import { contrastRatio, deltaEOK, parseColor } from '../colour'

describe('colour', () => {
  it('FR-DIFF-11 contrastRatio of white against black is 21:1', () => {
    expect(contrastRatio('#ffffff', '#000000')).toBeCloseTo(21, 1)
  })

  it('FR-DIFF-11 deltaEOK of white against black is 1.0', () => {
    expect(deltaEOK('#ffffff', '#000000')).toBeCloseTo(1, 2)
  })

  it('FR-DIFF-11 parseColor reads color(srgb 1 1 1) as opaque white', () => {
    expect(parseColor('color(srgb 1 1 1)')).toEqual({ r: 255, g: 255, b: 255, a: 1 })
  })

  it("FR-DIFF-11 parseColor reads a translucent colour's alpha", () => {
    expect(parseColor('rgba(10, 20, 30, 0.25)')).toEqual({ r: 10, g: 20, b: 30, a: 0.25 })
    expect(parseColor('color(srgb 0 0 1 / 0.5)')).toEqual({ r: 0, g: 0, b: 255, a: 0.5 })
  })
})
