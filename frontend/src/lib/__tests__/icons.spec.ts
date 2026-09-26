import { globSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { statusIcon } from '../icons'
import { i18n } from '@/i18n'

const t = i18n.global.t

describe('statusIcon', () => {
  it.each([
    ['A', 'file-plus', 'Added', 'green'],
    ['M', 'file-edit', 'Modified', 'blue'],
    ['D', 'trash', 'Deleted', 'red'],
    ['R', 'file-export', 'Renamed', 'violet'],
    ['C', 'copy', 'Copied', 'violet'],
    ['T', 'sync', 'Type changed', 'muted'],
    ['?', 'question-circle', 'Untracked', 'muted'],
    ['U', 'exclamation-triangle', 'Conflicted', 'orange'],
  ])('FR-UX-13 status %s maps to its icon, label and tone', (status, icon, label, tone) => {
    expect(statusIcon(status, t)).toEqual({ icon, label, tone })
  })

  it('FR-UX-13 a two-letter conflict code (as git reports one) maps to Conflicted', () => {
    expect(statusIcon('AU', t)).toEqual({
      icon: 'exclamation-triangle',
      label: 'Conflicted',
      tone: 'orange',
    })
  })

  it('FR-UX-13 an unrecognised status falls back to a file icon labelled with the code itself', () => {
    expect(statusIcon('X', t)).toEqual({ icon: 'file', label: 'X', tone: 'muted' })
  })
})

// Every .vue template under src/ draws its icons from lib/icons.ts's
// classes, never a symbol or emoji glyph, an HTML entity standing in for
// one, or its own oi- class string. The right arrow and the ellipsis are
// ordinary running-text punctuation ("old → new", "New branch…"), not icons.
// The glob is relative to the frontend package root, where vitest runs.
const files = globSync('src/**/*.vue')

const ALLOWED_SYMBOLS = new Set(['→', '…'])
const SYMBOL_OR_EMOJI = /\p{S}|\p{Extended_Pictographic}/u

function isIconGlyph(ch: string): boolean {
  return ch.codePointAt(0)! > 0x7f && SYMBOL_OR_EMOJI.test(ch) && !ALLOWED_SYMBOLS.has(ch)
}

// A glyph used as an icon is either the sole content of a quoted string (a
// map value, a ternary branch) or the sole content between two tags;
// elsewhere the same character is ordinary text (a name, a badge).
function quotedGlyphs(content: string): string[] {
  return [...content.matchAll(/(['"])(.)\1/gu)].map((m) => m[2]!).filter(isIconGlyph)
}
function bareGlyphs(content: string): string[] {
  return [...content.matchAll(/>\s*(\S)\s*</gu)].map((m) => m[1]!).filter(isIconGlyph)
}

// An HTML entity decodes to one character regardless of where it sits, so
// entities are checked wherever they appear, not just as sole content.
const NAMED_ENTITY: Record<string, string> = {
  times: '×',
  rarr: '→',
  larr: '←',
  uarr: '↑',
  darr: '↓',
  hellip: '…',
  mdash: '—',
  ndash: '–',
  bull: '•',
  middot: '·',
}
function decodeEntity(body: string): string | undefined {
  if (body[0] !== '#') return NAMED_ENTITY[body.toLowerCase()]
  const hex = body[1] === 'x' || body[1] === 'X'
  const code = parseInt(body.slice(hex ? 2 : 1), hex ? 16 : 10)
  return String.fromCodePoint(code)
}
function entityGlyphs(content: string): string[] {
  return [...content.matchAll(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi)]
    .map((m) => decodeEntity(m[1]!))
    .filter((ch): ch is string => !!ch && isIconGlyph(ch))
}

const OI_CLASS = /\boi-[a-z0-9-]+/

describe('TR-34 icon guard', () => {
  it('TR-34 no template uses a symbol glyph, emoji or HTML entity as an icon', () => {
    const offenders = files.filter((f) => {
      const content = readFileSync(f, 'utf-8')
      return (
        quotedGlyphs(content).length > 0 ||
        bareGlyphs(content).length > 0 ||
        entityGlyphs(content).length > 0
      )
    })
    expect(offenders).toEqual([])
  })

  it('TR-34 no template spells its own oi- class instead of using lib/icons.ts', () => {
    const offenders = files.filter((f) => OI_CLASS.test(readFileSync(f, 'utf-8')))
    expect(offenders).toEqual([])
  })
})
