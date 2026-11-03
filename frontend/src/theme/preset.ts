import { definePreset } from '@openuxkit/themes'
import Aura from '@openuxkit/themes/aura'

// NebegitPreset is Aura with the app's design-token overrides, the one place
// OpenVue's look may change.
export const NebegitPreset = definePreset(Aura, {
  components: {
    // Aura draws a severity's toast as its colour mixed with transparent,
    // plus a blur, so the window behind shows through. Here the same colour
    // is mixed with the content background instead: the same look, opaque.
    toast: {
      colorScheme: {
        light: {
          root: { blur: '0' },
          info: { background: 'color-mix(in srgb, {blue.50} 95%, {surface.0})' },
          success: { background: 'color-mix(in srgb, {green.50} 95%, {surface.0})' },
          warn: { background: 'color-mix(in srgb, {yellow.50} 95%, {surface.0})' },
          error: { background: 'color-mix(in srgb, {red.50} 95%, {surface.0})' },
        },
        dark: {
          root: { blur: '0' },
          info: { background: 'color-mix(in srgb, {blue.500} 16%, {surface.900})' },
          success: { background: 'color-mix(in srgb, {green.500} 16%, {surface.900})' },
          warn: { background: 'color-mix(in srgb, {yellow.500} 16%, {surface.900})' },
          error: { background: 'color-mix(in srgb, {red.500} 16%, {surface.900})' },
        },
      },
    },
  },
})
