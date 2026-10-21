// Requirements: FR-UX-08
import { describe, expect, it, afterEach } from 'vitest'
import { useLayout } from '../layout'

const DEFAULT_WIDTH = 1024

function setInnerWidth(width: number): void {
  window.innerWidth = width
  window.dispatchEvent(new Event('resize'))
}

describe('useLayout', () => {
  afterEach(() => setInnerWidth(DEFAULT_WIDTH))

  it('reports wide at 1400px and above', () => {
    setInnerWidth(1400)
    expect(useLayout().value).toBe('wide')
  })

  it('reports medium between 900px and 1400px', () => {
    setInnerWidth(900)
    expect(useLayout().value).toBe('medium')

    setInnerWidth(1399)
    expect(useLayout().value).toBe('medium')
  })

  it('reports narrow below 900px', () => {
    setInnerWidth(899)
    expect(useLayout().value).toBe('narrow')
  })

  it('updates reactively as the window is resized', () => {
    // Starts narrow: happy-dom's MediaQueryList assumes a listener starts
    // unmatched, so the first real transition must be upward from there for
    // every later resize, either way, to be detected.
    setInnerWidth(600)
    const layout = useLayout()
    expect(layout.value).toBe('narrow')

    setInnerWidth(1600)
    expect(layout.value).toBe('wide')

    setInnerWidth(600)
    expect(layout.value).toBe('narrow')
  })
})
