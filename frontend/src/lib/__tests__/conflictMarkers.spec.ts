import { describe, expect, it } from 'vitest'
import {
  findBlocks,
  hasMarkerLines,
  isMarkerLine,
  takeLines,
  type MarkerBlock,
} from '../conflictMarkers'

const merge = ['a', '<<<<<<< HEAD', 'cur1', 'cur2', '=======', 'inc1', '>>>>>>> side', 'z'].join(
  '\n',
)
const diff3 = ['<<<<<<< HEAD', 'cur', '||||||| base', 'old', '=======', 'inc', '>>>>>>> side'].join(
  '\n',
)

describe('conflictMarkers', () => {
  it('FR-CONF-05 finds a merge-style block with its sections', () => {
    expect(findBlocks(merge, 7)).toEqual([
      { startLine: 2, endLine: 7, current: ['cur1', 'cur2'], base: null, incoming: ['inc1'] },
    ])
  })

  it('FR-CONF-05 finds diff3 and zdiff3 blocks with a base section', () => {
    expect(findBlocks(diff3, 7)).toEqual([
      { startLine: 1, endLine: 7, current: ['cur'], base: ['old'], incoming: ['inc'] },
    ])
    const zdiff3 = [
      '<<<<<<< HEAD',
      'same',
      'cur',
      '||||||| base',
      'old',
      '=======',
      'inc',
      'same',
      '>>>>>>> s',
    ].join('\n')
    expect(findBlocks(zdiff3, 7)[0]).toMatchObject({
      current: ['same', 'cur'],
      base: ['old'],
      incoming: ['inc', 'same'],
    })
  })

  it('FR-CONF-05 supports other marker sizes and empty sections', () => {
    const text = ['<<<<<<<<<< a', '==========', 'x', '>>>>>>>>>>'].join('\n')
    expect(findBlocks(text, 10)).toEqual([
      { startLine: 1, endLine: 4, current: [], base: null, incoming: ['x'] },
    ])
    expect(findBlocks(text, 7)).toEqual([])
  })

  it('FR-CONF-05 finds several blocks', () => {
    expect(findBlocks(`${merge}\n${merge}`, 7).map((b) => [b.startLine, b.endLine])).toEqual([
      [2, 7],
      [10, 15],
    ])
  })

  it('FR-CONF-05 ignores incomplete blocks and a lone separator', () => {
    expect(findBlocks('<<<<<<< a\nx\n=======\ny', 7)).toEqual([])
    expect(findBlocks('a\n=======\nb', 7)).toEqual([])
  })

  it('FR-CONF-05 recognises marker lines, stray ones included', () => {
    expect(isMarkerLine('<<<<<<< HEAD', 7)).toBe(true)
    expect(isMarkerLine('>>>>>>>', 7)).toBe(true)
    expect(isMarkerLine('<<<<<<<<', 7)).toBe(false)
    expect(isMarkerLine('<<<<<<x', 7)).toBe(false)
    expect(isMarkerLine('=======', 7)).toBe(false)
    expect(hasMarkerLines('a\n>>>>>>> x\nb', 7)).toBe(true)
    expect(hasMarkerLines('a\n=======\nb', 7)).toBe(false)
  })

  it('FR-CONF-05 splits lines at newline only', () => {
    const text = '<<<<<<< a\r\nx y\n=======\nz\n>>>>>>> b'
    expect(findBlocks(text, 7)[0]).toMatchObject({ current: ['x y'], incoming: ['z'] })
  })

  it('FR-CONF-05 builds each take as lines', () => {
    const block: MarkerBlock = {
      startLine: 1,
      endLine: 5,
      current: ['c'],
      base: ['b'],
      incoming: ['i1', 'i2'],
    }
    expect(takeLines(block, 'current')).toEqual(['c'])
    expect(takeLines(block, 'incoming')).toEqual(['i1', 'i2'])
    expect(takeLines(block, 'currentFirst')).toEqual(['c', 'i1', 'i2'])
    expect(takeLines(block, 'incomingFirst')).toEqual(['i1', 'i2', 'c'])
  })
})
