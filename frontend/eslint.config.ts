import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import eslintConfigPrettier from 'eslint-config-prettier'
import oxlint from 'eslint-plugin-oxlint'

// vue-tsc --build (composite project mode, TR-33) needs a nameable
// declaration type for every exported value; defineConfigWithVueTs's
// inferred return type resolves into pnpm's unstable .pnpm/ store path,
// which isn't portable, hence the explicit annotation.
const config: ReturnType<typeof defineConfigWithVueTs> = defineConfigWithVueTs(
  { ignores: ['dist/**', 'bindings/**', 'node_modules/**'] },
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommendedTypeChecked,
  eslintConfigPrettier,
  {
    rules: {
      // Matches tsconfig's noUnusedParameters, which already exempts a
      // leading underscore: mocked bindings (TR-31) keep unused positional
      // params to match the real Facade's call signature.
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  // Must be last: turns off the ESLint rules oxlint already covers, so it
  // has to win over everything before it.
  oxlint.configs['flat/recommended'],
)

export default config
