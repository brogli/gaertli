import { configDefaults, defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

// Specs that touch no DOM; every other spec runs in the DOM project.
const nodeSpecs = [
  'src/__tests__/i18n.spec.ts',
  'src/composables/__tests__/{useConfirmAction,useCoreCall,usePush}.spec.ts',
  'src/lib/__tests__/{codeEditor,conflict{Markers,Regions,Sides,Text},fileTree,format,gitHunks,graphLanes,icons,laneColours,openvueLocale,path}.spec.ts',
  'src/stores/__tests__/{changes,compare,log,refs,repo}.spec.ts',
  'src/testing/__tests__/colour.spec.ts',
]

// e2e/ and e2e-real/ hold Playwright tests, not Vitest's.
const exclude = [...configDefaults.exclude, 'e2e/**', 'e2e-real/**']

export default mergeConfig(
  viteConfig({ command: 'serve', mode: 'test' }),
  defineConfig({
    test: {
      projects: [
        {
          extends: true,
          test: { name: 'node', environment: 'node', include: nodeSpecs, exclude },
        },
        {
          extends: true,
          test: {
            name: 'dom',
            environment: 'happy-dom',
            setupFiles: ['src/testing/setup.ts'],
            exclude: [...exclude, ...nodeSpecs],
          },
        },
      ],
    },
  }),
)
