import { describe, expect, it } from 'vitest'
import { isMarkerLine } from '../conflictMarkers'

describe('conflictMarkers', () => {
  it('FR-CONF-05 recognises marker lines, stray ones included', () => {
    expect(isMarkerLine('<<<<<<< HEAD', 7)).toBe(true)
    expect(isMarkerLine('>>>>>>>', 7)).toBe(true)
    expect(isMarkerLine('>>>>>>>\r', 7)).toBe(true)
    expect(isMarkerLine('<<<<<<<<', 7)).toBe(false)
    expect(isMarkerLine('<<<<<<x', 7)).toBe(false)
    expect(isMarkerLine('=======', 7)).toBe(false)
  })

  it('FR-CONF-05 supports other marker sizes', () => {
    expect(isMarkerLine('<<<<<<<<<< a', 10)).toBe(true)
    expect(isMarkerLine('<<<<<<<<<< a', 7)).toBe(false)
  })
})
