import { configDefaults, defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

export default mergeConfig(
  viteConfig({ command: 'serve', mode: 'test' }),
  defineConfig({
    test: {
      environment: 'happy-dom',
      setupFiles: ['src/testing/setup.ts'],
      // e2e/*.spec.ts are Playwright tests, not Vitest's.
      exclude: [...configDefaults.exclude, 'e2e/**'],
    },
  }),
)
