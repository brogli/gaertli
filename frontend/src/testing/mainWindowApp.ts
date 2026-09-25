import { vi, type Mock } from 'vitest'
import { changedFilesFixture } from './fixtures/changes'
import { commitDetailsFixture } from './fixtures/commitDetails'
import { defaultSettingsFixture } from './fixtures/settings'
import { summaryFixture } from './fixtures/summary'

// Stand-ins for the app bindings the main window's views (MainView,
// WorktreeContent and their children) call. A test file routes the module
// here:
//
//   vi.mock(
//     '../../bindings/github.com/brogli/nebegit/internal/app',
//     () => import('../testing/mainWindowApp'),
//   )
export const Facade = mocks(
  'Summary',
  'LoadLog',
  'LogRows',
  'LogIndexOf',
  'CommitDetails',
  'Commit',
  'SetMessageDraft',
  'Refresh',
  'Refs',
  'CheckoutCommit',
  'UncommittedCount',
  'OpenFolder',
  'ListWorktrees',
  'Jobs',
  'CancelJob',
  'Status',
  'CompareCommits',
  'Version',
)
export const AppService = mocks(
  'UpdateWindowTitle',
  'OpenDiff',
  'NewWindow',
  'RecentRepos',
  'ListWSLDistros',
  'LogPath',
)
export const SettingsService = mocks('Get', 'Set')

// resetMainWindowApp answers every call as for a clean repo on main with no
// commits, refs, worktrees, jobs or recent repos.
export function resetMainWindowApp(): void {
  for (const service of [Facade, AppService, SettingsService])
    for (const fn of Object.values(service)) fn.mockReset().mockResolvedValue(undefined)
  Facade.Summary.mockResolvedValue(summaryFixture())
  Facade.LogRows.mockResolvedValue({ rows: [], loaded: 0, done: true })
  Facade.LogIndexOf.mockResolvedValue({ index: 0, found: false })
  Facade.CommitDetails.mockResolvedValue(commitDetailsFixture())
  Facade.Refs.mockResolvedValue({ branches: [], remotes: [], tags: [] })
  Facade.UncommittedCount.mockResolvedValue(0)
  Facade.ListWorktrees.mockResolvedValue([])
  Facade.Jobs.mockResolvedValue([])
  Facade.Status.mockResolvedValue(changedFilesFixture())
  Facade.CompareCommits.mockResolvedValue([])
  Facade.Version.mockResolvedValue({ app: 'dev', protocol: 1 })
  AppService.RecentRepos.mockResolvedValue([])
  AppService.ListWSLDistros.mockResolvedValue([])
  AppService.LogPath.mockResolvedValue({ path: '/mock/nebegit.log', error: '' })
  SettingsService.Get.mockResolvedValue(defaultSettingsFixture())
}

function mocks<Name extends string>(...names: Name[]): Record<Name, Mock> {
  return Object.fromEntries(names.map((name) => [name, vi.fn()])) as Record<Name, Mock>
}
