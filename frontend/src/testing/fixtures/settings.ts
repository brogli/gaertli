import { PullMode } from '../../../bindings/github.com/brogli/gaertli/internal/core'
import {
  DiffMode,
  Theme,
  type Preferences,
} from '../../../bindings/github.com/brogli/gaertli/internal/settings'

// defaultSettingsFixture is a fully-populated Preferences value, shared by
// every test that mocks SettingsService.Get.
export function defaultSettingsFixture(overrides: Partial<Preferences> = {}): Preferences {
  return {
    editorProfile: { name: '', folderCmd: '', fileCmd: '', fileAtLineCmd: '' },
    protectedBranches: [],
    defaultPullMode: PullMode.PullMerge,
    diffMode: DiffMode.UnifiedDiff,
    autoFetchIntervalMinutes: 0,
    theme: Theme.SystemTheme,
    ignoreWhitespace: false,
    debugMode: false,
    ...overrides,
  }
}
