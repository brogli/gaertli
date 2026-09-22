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
  },
  SettingsService: { Get: mockSettingsGet, Set: vi.fn() },
}))

import WorktreeView from './WorktreeView.vue'
import CreateBranchDialog from '../components/CreateBranchDialog.vue'
import ResetDialog from '../components/ResetDialog.vue'
import LogList from '../components/LogList.vue'
import CommitDetailsPanel from '../components/CommitDetailsPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import { useLogStore } from '../stores/log'
import { commitRow } from '../testing/fixtures/log-rows'
import { defaultSettingsFixture } from '../testing/fixtures/settings'

function mountView(props: { path: string }) {
  return mount(WorktreeView, { props })
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

// mountWithCheckoutCommitDialogOpen mounts WorktreeView with one log row,
// right-clicks it, and opens "Check out commit" from the resulting menu —
// the shared setup cases 21/22 (FR-BR-08) both need before diverging on
// confirm vs cancel.
async function mountWithCheckoutCommitDialogOpen() {
  const wrapper = mount(WorktreeView, {
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

describe('WorktreeView', () => {
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
    const wrapper = mount(WorktreeView, {
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

  it('FR-ED-06 an EDITOR_NOT_FOUND error opens the settings panel via ErrorBanner\'s link', async () => {
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
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent(SettingsPanel).exists()).toBe(true)
  })
})
