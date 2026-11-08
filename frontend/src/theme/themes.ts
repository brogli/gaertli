import { palette, type ColorScale } from '@openuxkit/themes'
import { semantic as auraSemantic } from '@openuxkit/themes/aura/base'
// From @openuxkit/styled, not @openuxkit/themes: the latter re-exports them
// with types that reject the { dark } palette form and palette()'s result.
import { updatePreset, updatePrimaryPalette, updateSurfacePalette } from '@openuxkit/styled'
import { Theme } from '../../bindings/github.com/brogli/gaertli/internal/settings'

export interface ThemeCredit {
  name: string
  copyright: string
  licence: string
  url: string
}

export interface DarkThemeEntry {
  id: Theme
  label: string
  accent: string
  surface: ColorScale
  credit: ThemeCredit
}

// darkThemes are the four themes beyond System, Light and Dark. Their
// palettes are hand-written from each project's official colours; steps a
// theme lacks are sRGB blends of their neighbours.
export const darkThemes: DarkThemeEntry[] = [
  {
    id: Theme.GruvboxDarkTheme,
    label: 'Gruvbox dark',
    accent: '#fe8019',
    surface: {
      0: '#ebdbb2',
      50: '#ebdbb2',
      100: '#d5c4a1',
      200: '#bdae93',
      300: '#a89984',
      400: '#928374',
      500: '#7c6f64',
      600: '#665c54',
      700: '#504945',
      800: '#3c3836',
      900: '#282828',
      950: '#1d2021',
    },
    credit: {
      name: 'Gruvbox',
      copyright: 'Copyright (c) morhetz',
      licence: 'MIT',
      url: 'https://github.com/morhetz/gruvbox',
    },
  },
  {
    id: Theme.CatppuccinMochaTheme,
    label: 'Catppuccin Mocha',
    accent: '#cba6f7',
    surface: {
      0: '#cdd6f4',
      50: '#cdd6f4',
      100: '#bac2de',
      200: '#a6adc8',
      300: '#9399b2',
      400: '#7f849c',
      500: '#6c7086',
      600: '#585b70',
      700: '#45475a',
      800: '#313244',
      900: '#1e1e2e',
      950: '#181825',
    },
    credit: {
      name: 'Catppuccin',
      copyright: 'Copyright (c) 2021 Catppuccin',
      licence: 'MIT',
      url: 'https://github.com/catppuccin/palette',
    },
  },
  {
    id: Theme.NordTheme,
    label: 'Nord',
    accent: '#88c0d0',
    surface: {
      0: '#eceff4',
      50: '#eceff4',
      100: '#e5e9f0',
      200: '#d8dee9',
      300: '#b5bcc9',
      400: '#929aaa',
      500: '#6f788a',
      600: '#4c566a',
      700: '#434c5e',
      800: '#3b4252',
      900: '#2e3440',
      950: '#242933',
    },
    credit: {
      name: 'Nord',
      copyright: 'Copyright (c) 2016-present Sven Greb',
      licence: 'MIT',
      url: 'https://github.com/nordtheme/nord',
    },
  },
  {
    id: Theme.DraculaTheme,
    label: 'Dracula',
    accent: '#bd93f9',
    surface: {
      0: '#f8f8f2',
      50: '#f8f8f2',
      100: '#d2d6de',
      200: '#adb5cb',
      300: '#8894b8',
      400: '#6272a4',
      500: '#5a6792',
      600: '#535c7f',
      700: '#4c526c',
      800: '#44475a',
      900: '#282a36',
      950: '#21222c',
    },
    credit: {
      name: 'Dracula',
      copyright: 'Copyright (c) 2023 Dracula Theme',
      licence: 'MIT',
      url: 'https://github.com/dracula/dracula-theme',
    },
  },
]

// Aura's own accent and dark surface, restored for System, Light and Dark.
const auraPrimary = auraSemantic.primary
const auraDarkSurface = auraSemantic.colorScheme?.dark?.surface
const auraDarkPrimaryColor = auraSemantic.colorScheme?.dark?.primary?.color

// applyTheme sets the .gaertli-dark class for the setting and the OS
// preference, and swaps @openuxkit/themes' primary and dark surface
// palettes: a dark theme's hand-written palette, or Aura's own otherwise.
export function applyTheme(setting: Theme, osPrefersDark: boolean): void {
  const theme = darkThemes.find((t) => t.id === setting)
  const dark =
    theme !== undefined ||
    setting === Theme.DarkTheme ||
    (setting !== Theme.LightTheme && osPrefersDark)
  document.documentElement.classList.toggle('gaertli-dark', dark)

  updatePrimaryPalette(theme ? palette(theme.accent) : auraPrimary)
  updateSurfacePalette({ dark: theme ? theme.surface : auraDarkSurface })
  updatePreset({
    semantic: {
      colorScheme: { dark: { primary: { color: theme ? theme.accent : auraDarkPrimaryColor } } },
    },
  })
}
