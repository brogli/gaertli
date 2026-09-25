import { globSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// Every path here is relative to the frontend package root, where vitest
// runs.

// Read as plain JSON, not `import ... from './locales/en.json'`: the
// unplugin-vue-i18n Vite plugin precompiles that import into message
// functions for vue-i18n itself, not the plain string tree this guard reads.
const en: unknown = JSON.parse(readFileSync('src/locales/en.json', 'utf-8'))

// Proper names the guard lets through as plain text: the app's own name,
// git itself, and the detached-HEAD state.
const ALLOWED_WORDS = new Set(['nebegit', 'git', 'HEAD'])

const ATTRS = ['title', 'aria-label', 'placeholder', 'label', 'header']

// hasRealText: after removing mustache expressions and HTML entities (&rarr;
// and the like decode to punctuation, not words), does any word remain that
// isn't an allowed proper name? A Vue interpolation ({{ ... }}) is already
// backed by a store's or a component's own t() call, which this text-level
// scan cannot see into; only the literal characters around it matter here.
function hasRealText(raw: string): boolean {
  const stripped = raw.replace(/\{\{[\s\S]*?\}\}/g, ' ').replace(/&[a-zA-Z0-9#]+;/g, ' ')
  const words = stripped.match(/\p{L}+/gu) ?? []
  return words.some((w) => !ALLOWED_WORDS.has(w))
}

function templateOf(source: string): string {
  const start = source.indexOf('<template')
  const end = source.lastIndexOf('</template>')
  if (start === -1 || end === -1) return ''
  return source.slice(source.indexOf('>', start) + 1, end).replace(/<!--[\s\S]*?-->/g, '')
}

// blankAttrValues replaces every double-quoted attribute value's content
// with spaces, keeping every other character (and the string's length, so
// offsets still line up with the original) in place. A directive's JS
// expression can itself contain "<" or ">" (an arrow function, a
// comparison); left in, those break tag/text-node detection below.
function blankAttrValues(template: string): string {
  return template.replace(/="([^"]*)"/g, (_m, value: string) => `="${' '.repeat(value.length)}"`)
}

// textNodeOffenders finds literal text sitting directly between two tags.
function textNodeOffenders(template: string): string[] {
  const blanked = blankAttrValues(template)
  return [...blanked.matchAll(/>([^<]+)</g)]
    .map((m) => m[1]!)
    .filter((text) => text.trim() && hasRealText(text))
}

// attrOffenders finds an untranslated literal (never a v-bind:/: expression,
// which cannot be checked this way) value for one of ATTRS. Tag boundaries
// come from the blanked template (see above); the attribute values checked
// are read back from the same span of the original, unblanked template.
function attrOffenders(template: string): string[] {
  const blanked = blankAttrValues(template)
  const offenders: string[] = []
  for (const m of blanked.matchAll(/<[a-zA-Z][^>]*>/g)) {
    const tag = template.slice(m.index, m.index + m[0].length)
    for (const attr of ATTRS) {
      const re = new RegExp(`(?:^|\\s)${attr}="([^"]*)"`)
      const found = re.exec(tag)
      if (found && hasRealText(found[1]!)) offenders.push(`${attr}="${found[1]}"`)
    }
  }
  return offenders
}

describe('TR-35 translation guard', () => {
  it('TR-35 no .vue template holds untranslated literal text or attribute', () => {
    const offenders = globSync('src/**/*.vue').flatMap((file) => {
      const template = templateOf(readFileSync(file, 'utf-8'))
      const found = [...textNodeOffenders(template), ...attrOffenders(template)]
      return found.map((text) => `${file}: ${text}`)
    })
    expect(offenders).toEqual([])
  })

  it('FR-UX-11 every literal t()/$t() key used in the source exists in en.json', () => {
    const keyPattern = /(?<![\w$])\$?t\(\s*(['"])([a-zA-Z0-9_.]+)\1/g
    const files = globSync('src/**/*.{ts,vue}').filter((f) => !f.endsWith('.test.ts'))
    const missing: string[] = []
    for (const file of files) {
      const content = readFileSync(file, 'utf-8')
      for (const m of content.matchAll(keyPattern)) {
        const key = m[2]!
        const value = key.split('.').reduce<unknown>((obj, part) => {
          return obj && typeof obj === 'object' ? (obj as Record<string, unknown>)[part] : undefined
        }, en)
        if (typeof value !== 'string') missing.push(`${file}: ${key}`)
      }
    }
    expect(missing).toEqual([])
  })
})
