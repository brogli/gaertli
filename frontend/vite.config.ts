import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite'

export default defineConfig(({ mode }) => ({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  // Precompiles src/locales/*.json to message functions and bundles
  // vue-i18n's runtime-only build on a production build, so no message
  // string is ever compiled with eval at runtime (CSP-safe).
  plugins: [
    vue(),
    tailwindcss(),
    wails('./bindings'),
    VueI18nPlugin({
      include: [fileURLToPath(new URL('./src/locales/**', import.meta.url))],
    }),
  ],
  // e2e mode: swap the generated internal/app bindings (Facade, AppService,
  // SettingsService) for a mock backed by canned fixtures, so Playwright
  // drives the real Vue app without a live Go core. A RegExp match, not a
  // literal path, because the bindings import's `../..` prefix differs by
  // caller depth.
  resolve: {
    alias: [
      { find: '@', replacement: fileURLToPath(new URL('./src', import.meta.url)) },
      ...(mode === 'e2e'
        ? [
            {
              find: /(\.\.\/)+bindings\/github\.com\/brogli\/nebegit\/internal\/app$/,
              replacement: fileURLToPath(
                new URL('./src/testing/e2e-mock-bindings.ts', import.meta.url),
              ),
            },
          ]
        : []),
    ],
  },
}))
