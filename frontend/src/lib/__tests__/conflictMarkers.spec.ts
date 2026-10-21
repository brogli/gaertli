// Requirements: FR-CONF-05
import { describe, expect, it } from 'vitest'
import { isMarkerLine } from '../conflictMarkers'

describe('conflictMarkers', () => {
  it('recognises marker lines, stray ones included', () => {
    expect(isMarkerLine('<<<<<<< HEAD', 7)).toBe(true)
    expect(isMarkerLine('>>>>>>>', 7)).toBe(true)
    expect(isMarkerLine('>>>>>>>\r', 7)).toBe(true)
    expect(isMarkerLine('<<<<<<<<', 7)).toBe(false)
    expect(isMarkerLine('<<<<<<x', 7)).toBe(false)
    expect(isMarkerLine('=======', 7)).toBe(false)
  })

  it('supports other marker sizes', () => {
    expect(isMarkerLine('<<<<<<<<<< a', 10)).toBe(true)
    expect(isMarkerLine('<<<<<<<<<< a', 7)).toBe(false)
  })
})
