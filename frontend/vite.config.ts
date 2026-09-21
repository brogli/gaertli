import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import wails from '@wailsio/runtime/plugins/vite'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [vue(), wails('./bindings')],
  // e2e mode (TR-31, spec 13): swap the generated Wails Facade for a mock
  // backed by a canned fixture, so Playwright drives the real Vue app
  // without a live Go core. A RegExp match, not a literal path, because the
  // bindings import's `../..` prefix differs by caller depth.
  resolve: {
    alias:
      mode === 'e2e'
        ? [
            {
              find: /(\.\.\/)+bindings\/github\.com\/brogli\/nebegit\/internal\/app$/,
              replacement: fileURLToPath(new URL('./src/testing/e2e-mock-bindings.ts', import.meta.url)),
            },
          ]
        : [],
  },
}))
