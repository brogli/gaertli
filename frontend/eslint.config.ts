import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import eslintConfigPrettier from 'eslint-config-prettier'
import oxlint from 'eslint-plugin-oxlint'
import pluginPlaywright from 'eslint-plugin-playwright'
import pluginVitest from '@vitest/eslint-plugin'
import eslintComments from '@eslint-community/eslint-plugin-eslint-comments'
import type { Linter } from 'eslint'

// crossFolderImports refuses a relative import that climbs `ups` folders
// or more, unless it leads to bindings/.
function crossFolderImports(ups: number): Linter.RuleEntry {
  const regex = String.raw`^(\.\./){${ups}}(?!(\.\./)*bindings/)`
  return ['error', { patterns: [{ regex, message: 'Import from another src/ folder with @/…' }] }]
}

// vue-tsc --build (composite project mode) needs a nameable
// declaration type for every exported value; defineConfigWithVueTs's
// inferred return type resolves into pnpm's unstable .pnpm/ store path,
// which isn't portable, hence the explicit annotation.
const config: ReturnType<typeof defineConfigWithVueTs> = defineConfigWithVueTs(
  { ignores: ['dist/**', 'bindings/**', 'node_modules/**'] },
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommendedTypeChecked,
  {
    ...pluginPlaywright.configs['flat/recommended'],
    files: ['e2e/**/*.spec.ts', 'e2e-real/**/*.spec.ts'],
    rules: {
      ...pluginPlaywright.configs['flat/recommended'].rules,
      // Helpers that assert on the calling test's behalf.
      'playwright/expect-expect': [
        'warn',
        { assertFunctionNames: ['assertToolbarFitsOneRow', 'expectRowsFit', 'expectSubjects'] },
      ],
    },
  },
  {
    ...pluginVitest.configs.recommended,
    files: ['src/**/__tests__/*'],
  },
  // No `!` or `as` cast without a narrowing, a type guard or a reason; every
  // export and component declares its types. Tests and their helpers may
  // assert non-null: a wrong assumption throws and fails the test.
  {
    files: ['src/**'],
    ignores: ['src/**/__tests__/**', 'src/testing/**'],
    rules: { '@typescript-eslint/no-non-null-assertion': 'error' },
  },
  {
    files: ['src/**', 'e2e/**', 'e2e-real/**'],
    rules: {
      '@typescript-eslint/consistent-type-assertions': ['error', { assertionStyle: 'never' }],
      '@typescript-eslint/explicit-module-boundary-types': 'error',
      'vue/define-props-declaration': ['error', 'type-based'],
      'vue/define-emits-declaration': ['error', 'type-based'],
    },
  },
  // Native controls have no OpenVue look or keyboard handling; use the
  // matching component instead.
  {
    files: ['src/**/*.vue'],
    rules: {
      'vue/no-restricted-html-elements': [
        'error',
        {
          element: ['button', 'input', 'select', 'textarea'],
          message: 'Use the OpenVue component.',
        },
      ],
    },
  },
  // Outside data stays unknown until narrowed. Vue's config turns these rules
  // off because .vue files get fallback types with any; app .ts modules
  // (stores, lib, composables) get them back. Tests keep them off: Vue Test
  // Utils types a component instance as any.
  {
    files: ['src/**/*.ts'],
    ignores: ['src/**/__tests__/**', 'src/testing/**'],
    rules: {
      '@typescript-eslint/no-unsafe-argument': 'error',
      '@typescript-eslint/no-unsafe-assignment': 'error',
      '@typescript-eslint/no-unsafe-call': 'error',
      '@typescript-eslint/no-unsafe-member-access': 'error',
      '@typescript-eslint/no-unsafe-return': 'error',
    },
  },
  // Store actions declare their return type; parameters are already typed by
  // noImplicitAny.
  {
    files: ['src/stores/**'],
    ignores: ['src/stores/__tests__/**'],
    rules: {
      '@typescript-eslint/explicit-function-return-type': [
        'error',
        { allowExpressions: true, allowTypedFunctionExpressions: true },
      ],
    },
  },
  {
    plugins: { '@eslint-community/eslint-comments': eslintComments },
    linterOptions: { reportUnusedDisableDirectives: 'error' },
    rules: { '@eslint-community/eslint-comments/require-description': 'error' },
  },
  eslintConfigPrettier,
  {
    rules: {
      // Matches tsconfig's noUnusedParameters, which already exempts a
      // leading underscore: mocked bindings keep unused positional
      // params to match the real Facade's call signature.
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  // An import from another src/ folder uses the @ alias; bindings/ (outside
  // src/) has none. A __tests__ file sits one folder deeper, so the code it
  // tests is still a single '../' up.
  {
    files: ['src/**'],
    ignores: ['src/**/__tests__/**'],
    rules: { 'no-restricted-imports': crossFolderImports(1) },
  },
  {
    files: ['src/**/__tests__/**'],
    rules: { 'no-restricted-imports': crossFolderImports(2) },
  },
  // Must be last: turns off the ESLint rules oxlint already covers, so it
  // has to win over everything before it.
  oxlint.configs['flat/recommended'],
)

export default config
