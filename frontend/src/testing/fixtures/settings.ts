import type { Settings } from '../../../bindings/github.com/brogli/nebegit/internal/settings'

// defaultSettingsFixture is a fully-populated Settings value, shared by
// every test that mocks SettingsService.Get so a Settings-shape change
// (spec 31: EditorProfiles replacing the flat editor string) needs
// updating in one place, not once per test file.
export function defaultSettingsFixture(overrides: Partial<Settings> = {}): Settings {
  return {
    recentRepos: [],
    openWorktrees: [],
    editorProfiles: {},
    protectedBranches: [],
    defaultPullMode: 'merge',
    diffMode: 'unified',
    autoFetchIntervalMinutes: 0,
    ...overrides,
  }
}
