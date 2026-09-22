import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  mockSummary,
  mockCommandLog,
  mockLoadLog,
  mockLogRows,
  mockRefresh,
  mockRefs,
  mockCheckoutCommit,
  mockUncommittedCount,
  mockOpenFolder,
  mockSettingsGet,
  mockUpdateWindowTitle,
  mockListBranches,
  mockJobs,
  mockStatus,
  mockDiffFile,
  mockDiffWorking,
  mockEventsOn,
} = vi.hoisted(() => ({
  mockSummary: vi.fn(),
  mockCommandLog: vi.fn(),
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockRefresh: vi.fn(),
  mockRefs: vi.fn(),
  mockCheckoutCommit: vi.fn(),
  mockUncommittedCount: vi.fn(),
  mockOpenFolder: vi.fn(),
  mockSettingsGet: vi.fn(),
  mockUpdateWindowTitle: vi.fn(),
  mockListBranches: vi.fn(),
  mockJobs: vi.fn(),
  mockStatus: vi.fn(),
  mockDiffFile: vi.fn(),
  mockDiffWorking: vi.fn(),
  mockEventsOn: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    Summary: mockSummary,
    CommandLog: mockCommandLog,
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: vi.fn(),
    Refresh: mockRefresh,
    Refs: mockRefs,
    CheckoutCommit: mockCheckoutCommit,
    UncommittedCount: mockUncommittedCount,
    OpenFolder: mockOpenFolder,
    ListBranches: mockListBranches,
    Jobs: mockJobs,
    CancelJob: vi.fn(),
    Status: mockStatus,
    DiffFile: mockDiffFile,
    DiffWorking: mockDiffWorking,
  },
  AppService: { UpdateWindowTitle: mockUpdateWindowTitle },
  SettingsService: { Get: mockSettingsGet, Set: vi.fn() },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
}))

import WorktreeContent from './WorktreeContent.vue'
import CreateBranchDialog from '../components/CreateBranchDialog.vue'
import ResetDialog from '../components/ResetDialog.vue'
import LogList from '../components/LogList.vue'
import CommitDetailsPanel from '../components/CommitDetailsPanel.vue'
import CreateWorktreeDialog from '../components/CreateWorktreeDialog.vue'
import ChangesPanel from '../components/ChangesPanel.vue'
import DiffView from '../components/DiffView.vue'
import SyncControls from '../components/SyncControls.vue'
import { useLogStore } from '../stores/log'
import { useChangesStore } from '../stores/changes'
import { commitRow } from '../testing/fixtures/log-rows'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { changedFilesFixture } from '../testing/fixtures/changes'

function mountView(props: { path: string }) {
  return mount(WorktreeContent, { props })
}

function branch(fields: Record<string, unknown>) {
  return {
    name: 'main',
    sha: 'sha-main',
    current: true,
    upstream: '',
    ahead: 0,
    behind: 0,
    upstreamGone: false,
    checkedOutIn: '',
    ...fields,
  }
}

const sampleRow = commitRow()

// mountWithCheckoutCommitDialogOpen mounts WorktreeContent with one log row,
// right-clicks it, and opens "Check out commit" from the resulting menu —
// the setup FR-BR-08's confirm and cancel cases share.
async function mountWithCheckoutCommitDialogOpen() {
  const wrapper = mount(WorktreeContent, {
    props: { path: '/home/user/project' },
    attachTo: document.body,
  })
  await flushPromises()
  const log = useLogStore()
  log.rowCache.set(0, sampleRow)
  log.loaded = 1
  await wrapper.vm.$nextTick()

  await wrapper.get('.row').trigger('contextmenu')
  await wrapper.vm.$nextTick()
  const body = new DOMWrapper(document.body)
  const items = body.findAll('.p-contextmenu-item-content')
  await items[3]!.trigger('click') // "Check out commit"
  await wrapper.vm.$nextTick()

  return { wrapper, body }
}

// clickMenuItemByLabel finds a context-menu item by its label text and
// clicks it — robust to a menu's own item order/length (mirrors
// LogList.test.ts's own helper).
async function clickMenuItemByLabel(body: DOMWrapper<Element>, label: string): Promise<void> {
  const labels = body.findAll('.p-contextmenu-item-label')
  const index = labels.findIndex((l) => l.text() === label)
  const items = body.findAll('.p-contextmenu-item-content')
  await items[index]!.trigger('click')
}

describe('WorktreeContent', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockSummary.mockReset()
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: 'main',
      detached: false,
      operation: '',
    })
    mockCommandLog.mockReset()
    mockCommandLog.mockResolvedValue([])
    mockLoadLog.mockReset()
    mockLoadLog.mockResolvedValue(undefined)
    mockLogRows.mockReset()
    mockLogRows.mockResolvedValue({ rows: [], loaded: 0, done: true })
    mockRefresh.mockReset()
    mockRefresh.mockResolvedValue(undefined)
    mockRefs.mockReset()
    mockRefs.mockResolvedValue({ branches: [], remotes: [], tags: [] })
    mockCheckoutCommit.mockReset()
    mockCheckoutCommit.mockResolvedValue(undefined)
    mockUncommittedCount.mockReset()
    mockUncommittedCount.mockResolvedValue(0)
    mockOpenFolder.mockReset()
    mockOpenFolder.mockResolvedValue(undefined)
    mockSettingsGet.mockReset()
    mockSettingsGet.mockResolvedValue(defaultSettingsFixture())
    mockUpdateWindowTitle.mockReset()
    mockUpdateWindowTitle.mockResolvedValue(undefined)
    mockListBranches.mockReset()
    mockListBranches.mockResolvedValue([{ name: 'feature', checkedOutIn: '' }])
    mockJobs.mockReset()
    mockJobs.mockResolvedValue([])
    mockStatus.mockReset()
    mockStatus.mockResolvedValue(changedFilesFixture())
    mockDiffFile.mockReset()
    mockDiffFile.mockResolvedValue({ path: '', binary: false, tooLarge: false })
    mockDiffWorking.mockReset()
    mockDiffWorking.mockResolvedValue({ path: '', binary: false, tooLarge: false })
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
  })

  it('FR-UX-08 renders the commit details in a column beside the log, not below it', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    const logColumn = wrapper.find('.log-column')
    const sideColumn = wrapper.find('.side-column')
    expect(logColumn.findComponent(LogList).exists()).toBe(true)
    expect(sideColumn.findComponent(CommitDetailsPanel).exists()).toBe(true)
    expect(logColumn.element.nextElementSibling).toBe(sideColumn.element)
  })

  it('FR-WT-09 shows the worktree name from the path prop', () => {
    expect(mountView({ path: '/home/user/project' }).text()).toContain('project')
  })

  it('FR-UX-01 shows the error message and reveals gitStderr behind show details', async () => {
    mockSummary.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { message: 'not a git repository', gitStderr: 'fatal: not a git repository' },
      }),
    )

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.text()).toContain('not a git repository')
    expect(wrapper.text()).not.toContain('fatal: not a git repository')

    await wrapper.get('.error-banner button').trigger('click')

    expect(wrapper.text()).toContain('fatal: not a git repository')
  })

  it('FR-REPO-04 clicking Refresh calls the Refresh binding with the worktree id', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const refreshButton = wrapper.findAll('button').find((b) => b.text() === 'Refresh')
    await refreshButton?.trigger('click')

    expect(mockRefresh).toHaveBeenCalledTimes(1)
    expect(mockRefresh).toHaveBeenCalledWith('/home/user/project')
  })

  it('FR-REPO-03 shows an ahead/behind indicator when the branch has an upstream', async () => {
    mockRefs.mockResolvedValue({
      branches: [branch({ upstream: 'origin/main', ahead: 2, behind: 1 })],
      remotes: [],
      tags: [],
    })

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const indicator = wrapper.get('[data-test="ahead-behind"]')
    expect(indicator.text()).toContain('2')
    expect(indicator.text()).toContain('1')
  })

  it('FR-REPO-03 shows no ahead/behind indicator without an upstream', async () => {
    mockRefs.mockResolvedValue({ branches: [branch({})], remotes: [], tags: [] })

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('FR-SYNC-01, FR-SYNC-02, FR-SYNC-03 renders SyncControls in the summary area', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.findComponent(SyncControls).exists()).toBe(true)
  })

  it('FR-BR-08 checking out a commit warns, then confirming calls CheckoutCommit once', async () => {
    const { wrapper, body } = await mountWithCheckoutCommitDialogOpen()

    expect(body.text()).toContain('detaches HEAD')

    await body.get('[data-test="detach-confirm"]').trigger('click')
    await flushPromises()

    expect(mockCheckoutCommit).toHaveBeenCalledTimes(1)
    expect(mockCheckoutCommit).toHaveBeenCalledWith('/home/user/project', 'abc123')

    wrapper.unmount()
  })

  it('FR-BR-08 shows "Create branch here" while detached, opening the dialog with HEAD\'s own sha', async () => {
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: '',
      detached: true,
      operation: '',
      sha: 'abc123',
    })
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const createHereButton = wrapper.findAll('button').find((b) => b.text() === 'Create branch here')
    expect(createHereButton).toBeTruthy()
    await createHereButton!.trigger('click')
    await wrapper.vm.$nextTick()

    const dialog = wrapper.findComponent(CreateBranchDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('sha')).toBe('abc123')

    wrapper.unmount()
  })

  it('FR-BR-08 shows no "Create branch here" button while attached to a branch', async () => {
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: 'main',
      detached: false,
      operation: '',
    })
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.findAll('button').find((b) => b.text() === 'Create branch here')).toBeUndefined()
  })

  it('FR-BR-08 cancelling the warning never calls CheckoutCommit and closes the dialog', async () => {
    const { wrapper, body } = await mountWithCheckoutCommitDialogOpen()

    await body.get('[data-test="detach-cancel"]').trigger('click')
    await flushPromises()

    expect(mockCheckoutCommit).not.toHaveBeenCalled()
    expect(body.find('[data-test="detach-confirm"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('FR-BR-02, FR-LOG-11 "Reset current branch to here…" opens ResetDialog with the row\'s sha and subject', async () => {
    const wrapper = mount(WorktreeContent, {
      props: { path: '/home/user/project' },
      attachTo: document.body,
    })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()
    const body = new DOMWrapper(document.body)
    const items = body.findAll('.p-contextmenu-item-content')
    await items[4]!.trigger('click') // "Reset current branch to here…"
    await wrapper.vm.$nextTick()

    const dialog = wrapper.findComponent(ResetDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('sha')).toBe('abc123')
    expect(dialog.props('subject')).toBe('Fix the bug')

    wrapper.unmount()
  })

  it('FR-ED-01 "Open in {name}" is labelled with the editor name and calls OpenFolder with the path', async () => {
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: 'main',
      detached: false,
      operation: '',
      editorName: 'another editor',
    })
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const button = wrapper.findAll('button').find((b) => b.text() === 'Open in another editor')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(mockOpenFolder).toHaveBeenCalledTimes(1)
    expect(mockOpenFolder).toHaveBeenCalledWith('/home/user/project')
  })

  it('FR-ED-06 an EDITOR_NOT_FOUND error emits open-settings via ErrorBanner\'s link', async () => {
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: 'main',
      detached: false,
      operation: '',
      editorName: 'the editor',
    })
    mockOpenFolder.mockRejectedValueOnce(
      Object.assign(new Error('no editor'), {
        cause: { message: 'no editor configured', code: 'EDITOR_NOT_FOUND' },
      }),
    )
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const openButton = wrapper.findAll('button').find((b) => b.text() === 'Open in the editor')
    await openButton!.trigger('click')
    await flushPromises()

    const settingsLink = wrapper.findAll('button').find((b) => b.text() === 'Open editor setting')
    expect(settingsLink).toBeTruthy()

    await settingsLink!.trigger('click')

    expect(wrapper.emitted('open-settings')).toHaveLength(1)
  })

  it('FR-WT-06, FR-LOG-11 "Create worktree here…" from the log opens the dialog pre-filled at that commit', async () => {
    const wrapper = mount(WorktreeContent, {
      props: { path: '/home/user/project' },
      attachTo: document.body,
    })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()
    const body = new DOMWrapper(document.body)
    await clickMenuItemByLabel(body, 'Create worktree here…')
    await flushPromises()

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({ newBranch: true, start: 'abc123' })

    wrapper.unmount()
  })

  it('FR-WT-06 "Create worktree here…" on a local branch in the refs sidebar opens the dialog on that branch', async () => {
    mockRefs.mockResolvedValue({
      branches: [branch({ name: 'feature', current: false })],
      remotes: [],
      tags: [],
    })
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const item = wrapper.findAll('[role="treeitem"]').find((n) => n.attributes('aria-label') === 'feature')
    await item?.find('.ref-node').trigger('contextmenu')
    await wrapper.vm.$nextTick()
    const body = new DOMWrapper(document.body)
    await clickMenuItemByLabel(body, 'Create worktree here…')
    await flushPromises()

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({ newBranch: false, branch: 'feature' })
  })

  it('FR-WT-06 "Create worktree here…" on a remote branch in the refs sidebar starts a new branch at its name', async () => {
    mockRefs.mockResolvedValue({
      branches: [],
      remotes: [{ name: 'origin', branches: [{ name: 'feature', sha: 'sha-feature' }] }],
      tags: [],
    })
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const item = wrapper.findAll('[role="treeitem"]').find((n) => n.attributes('aria-label') === 'feature')
    await item?.find('.ref-node').trigger('contextmenu')
    await wrapper.vm.$nextTick()
    const body = new DOMWrapper(document.body)
    await clickMenuItemByLabel(body, 'Create worktree here…')
    await flushPromises()

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({ newBranch: true, start: 'feature' })
  })

  // detailsWithFile seeds the log store's selected commit, one changed file.
  function detailsWithFile(path: string) {
    return {
      sha: 'abc123',
      parents: [],
      author: '',
      authorEmail: '',
      authorAt: 0,
      committer: '',
      committerEmail: '',
      committerAt: 0,
      body: '',
      files: [{ path, status: 'M' }],
    }
  }

  it('FR-UX-07 a changes-panel selection takes precedence over the commit diff', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const log = useLogStore()
    log.details = detailsWithFile('x.txt')
    log.selectedFile = { path: 'x.txt', status: 'M' }
    const changes = useChangesStore()
    changes.selectedFile = { path: 'a.txt', oldPath: '', staged: true }
    await wrapper.vm.$nextTick()

    const diffView = wrapper.findComponent(DiffView)
    expect(diffView.exists()).toBe(true)
    expect(diffView.props('working')).toEqual({ path: 'a.txt', oldPath: '', staged: true })
  })

  it('FR-UX-07 selecting a commit file clears the changes-panel selection and shows the commit diff', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const changes = useChangesStore()
    changes.selectedFile = { path: 'a.txt', oldPath: '', staged: true }
    const log = useLogStore()
    log.details = detailsWithFile('x.txt')
    await wrapper.vm.$nextTick()

    await wrapper.get('.commit-details li').trigger('click')
    await wrapper.vm.$nextTick()

    expect(changes.selectedFile).toBeNull()
    expect(log.selectedFile).toEqual({ path: 'x.txt', status: 'M' })
    expect(wrapper.findComponent(DiffView).props('working')).toBeNull()
  })

  it('FR-UX-07 the changes-column collapse control hides and shows the panel, staying visible itself', async () => {
    const wrapper = mount(WorktreeContent, { props: { path: '/home/user/project' }, attachTo: document.body })
    await flushPromises()

    expect(wrapper.findComponent(ChangesPanel).exists()).toBe(true)
    expect(wrapper.get('.changes-body').isVisible()).toBe(true)

    const collapseButton = wrapper.get('[data-test="changes-collapse"]')
    await collapseButton.trigger('click')

    expect(wrapper.get('.changes-body').isVisible()).toBe(false)
    expect(collapseButton.isVisible()).toBe(true)

    await collapseButton.trigger('click')

    expect(wrapper.get('.changes-body').isVisible()).toBe(true)
    wrapper.unmount()
  })

  it('FR-CHG-01 a repo:changed index-scope event calls changes.load again', async () => {
    const changes = useChangesStore()
    const loadSpy = vi.spyOn(changes, 'load')

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
    expect(loadSpy).toHaveBeenCalledTimes(1)

    const handlers = mockEventsOn.mock.calls.filter((call) => call[0] === 'repo:changed').map((call) => call[1])
    handlers.forEach((handler) => handler({ data: { scope: 'index' } }))
    await flushPromises()

    expect(loadSpy).toHaveBeenCalledTimes(2)
    expect(loadSpy).toHaveBeenLastCalledWith('/home/user/project')
    wrapper.unmount()
  })
})
