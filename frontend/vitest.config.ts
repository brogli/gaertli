import { configDefaults, defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

export default mergeConfig(
  viteConfig({ command: 'serve', mode: 'test' }),
  defineConfig({
    test: {
      environment: 'happy-dom',
      // e2e/*.spec.ts are Playwright tests (TR-31), not Vitest's.
      exclude: [...configDefaults.exclude, 'e2e/**'],
    },
  }),
)
