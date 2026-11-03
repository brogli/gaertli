// Requirements: FR-UX-06
import { describe, expect, it } from 'vitest'
import { Theme as ThemeService } from '@openuxkit/themes'
import { Theme } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
import { contrastRatio } from '@/testing/colour'
import { applyTheme, darkThemes } from '../themes'

describe('themes', () => {
  it('each theme meets WCAG AA contrast for its text and its accent button text', () => {
    for (const theme of darkThemes) {
      expect(contrastRatio(theme.surface[0]!, theme.surface[900]!)).toBeGreaterThanOrEqual(4.5)
      expect(contrastRatio(theme.surface[900]!, theme.accent)).toBeGreaterThanOrEqual(4.5)
    }
  })

  it('applyTheme swaps the preset to a dark theme, and back to Aura on Dark', () => {
    const nord = darkThemes.find((t) => t.id === Theme.NordTheme)!

    applyTheme(Theme.NordTheme, false)
    expect(ThemeService.getPreset().semantic.colorScheme.dark.surface[900]).toBe(nord.surface[900])
    expect(ThemeService.getPreset().semantic.colorScheme.dark.primary.color).toBe(nord.accent)

    applyTheme(Theme.DarkTheme, false)
    expect(ThemeService.getPreset().semantic.colorScheme.dark.surface[900]).toBe('{zinc.900}')
    expect(ThemeService.getPreset().semantic.colorScheme.dark.primary.color).toBe('{primary.400}')
  })
})
