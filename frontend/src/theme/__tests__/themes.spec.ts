import { describe, expect, it } from 'vitest'
import { Theme as ThemeService } from '@openuxkit/themes'
import { Theme } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
import { applyTheme, darkThemes } from '../themes'

// contrastRatio is the WCAG relative-luminance contrast ratio between two
// sRGB hex colours (1 to 21).
function contrastRatio(hexA: string, hexB: string): number {
  const a = relativeLuminance(hexA)
  const b = relativeLuminance(hexB)
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
}

function relativeLuminance(hex: string): number {
  const channels = [1, 3, 5].map((start) =>
    srgbToLinear(parseInt(hex.slice(start, start + 2), 16) / 255),
  )
  return 0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!
}

function srgbToLinear(c: number): number {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

describe('themes', () => {
  it('FR-UX-06 each theme meets WCAG AA contrast for its text and its accent button text', () => {
    for (const theme of darkThemes) {
      expect(contrastRatio(theme.surface[0]!, theme.surface[900]!)).toBeGreaterThanOrEqual(4.5)
      expect(contrastRatio(theme.surface[900]!, theme.accent)).toBeGreaterThanOrEqual(4.5)
    }
  })

  it('FR-UX-06 applyTheme swaps the preset to a dark theme, and back to Aura on Dark', () => {
    const nord = darkThemes.find((t) => t.id === Theme.NordTheme)!

    applyTheme(Theme.NordTheme, false)
    expect(ThemeService.getPreset().semantic.colorScheme.dark.surface[900]).toBe(nord.surface[900])
    expect(ThemeService.getPreset().semantic.colorScheme.dark.primary.color).toBe(nord.accent)

    applyTheme(Theme.DarkTheme, false)
    expect(ThemeService.getPreset().semantic.colorScheme.dark.surface[900]).toBe('{zinc.900}')
    expect(ThemeService.getPreset().semantic.colorScheme.dark.primary.color).toBe('{primary.400}')
  })
})
