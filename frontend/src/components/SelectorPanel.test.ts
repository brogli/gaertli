import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const {
  mockPickAndOpen,
  mockRecentRepos,
  mockOpenRecent,
  mockListWSLDistros,
  mockPickAndOpenWSL,
  mockOpenPath,
  mockSelectWorktree,
  mockListWorktrees,
  mockListBranches,
  mockCheckout,
  mockOpenFolderAt,
} = vi.hoisted(() => ({
  mockPickAndOpen: vi.fn(),
  mockRecentRepos: vi.fn(),
  mockOpenRecent: vi.fn(),
  mockListWSLDistros: vi.fn(),
  mockPickAndOpenWSL: vi.fn(),
  mockOpenPath: vi.fn(),
  mockSelectWorktree: vi.fn(),
  mockListWorktrees: vi.fn(),
  mockListBranches: vi.fn(),
  mockCheckout: vi.fn(),
  mockOpenFolderAt: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    PickAndOpen: mockPickAndOpen,
    RecentRepos: mockRecentRepos,
    OpenRecent: mockOpenRecent,
    ListWSLDistros: mockListWSLDistros,
    PickAndOpenWSL: mockPickAndOpenWSL,
    OpenPath: mockOpenPath,
    SelectWorktree: mockSelectWorktree,
  },
  Facade: {
    ListWorktrees: mockListWorktrees,
    ListBranches: mockListBranches,
    Checkout: mockCheckout,
    OpenFolderAt: mockOpenFolderAt,
  },
}))

const { mockEventsOn } = vi.hoisted(() => ({ mockEventsOn: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Events: { On: mockEventsOn } }))

import SelectorPanel from './SelectorPanel.vue'
import CreateWorktreeDialog from './CreateWorktreeDialog.vue'
import ErrorBanner from './ErrorBanner.vue'
import { worktreeFixture } from '../testing/fixtures/worktree'

// repoTreeItem returns the top-level treeitem labelled with path.
function repoTreeItem(wrapper: ReturnType<typeof mount>, path: string) {
  return wrapper.findAll('[role="treeitem"]').find((n) => n.attributes('aria-label') === path)
}

async function expandRepo(wrapper: ReturnType<typeof mount>, path: string): Promise<void> {
  await repoTreeItem(wrapper, path)!.get('.p-tree-node-toggle-button').trigger('click')
  await flushPromises()
}


describe('SelectorPanel', () => {
  let wslWrapper: VueWrapper | undefined

  beforeEach(() => {
    setActivePinia(createPinia())
    mockPickAndOpen.mockReset().mockResolvedValue(undefined)
    mockRecentRepos.mockReset().mockResolvedValue([])
    mockOpenRecent.mockReset().mockResolvedValue(undefined)
    mockListWSLDistros.mockReset().mockResolvedValue([])
    mockPickAndOpenWSL.mockReset().mockResolvedValue(undefined)
    mockOpenPath.mockReset().mockResolvedValue(undefined)
    mockSelectWorktree.mockReset().mockResolvedValue(undefined)
    mockListWorktrees.mockReset().mockResolvedValue([])
    mockListBranches.mockReset().mockResolvedValue([])
    mockCheckout.mockReset().mockResolvedValue(undefined)
    mockOpenFolderAt.mockReset().mockResolvedValue(undefined)
    mockEventsOn.mockReset().mockReturnValue(() => {})
  })

  afterEach(() => {
    wslWrapper?.unmount()
    wslWrapper = undefined
  })

  // openWSLDialog mounts SelectorPanel, clicks "Open WSL folder…", and
  // hands back a DOMWrapper on the dialog's teleported content; wslWrapper
  // is unmounted in afterEach.
  async function openWSLDialog(): Promise<DOMWrapper<HTMLElement>> {
    wslWrapper = mount(SelectorPanel, { props: { selectedPath: '' }, attachTo: document.body })
    await wslWrapper.get('[data-test="open-wsl-folder"]').trigger('click')
    await flushPromises()
    return new DOMWrapper(document.body)
  }

  it('FR-REPO-01 opens the folder picker on click', async () => {
    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })

    const openFolderButton = wrapper.findAll('button').find((b) => b.text() === 'Open folder…')
    await openFolderButton!.trigger('click')

    expect(mockPickAndOpen).toHaveBeenCalledTimes(1)
  })

  it('FR-REPO-02 lists recent repos and opens the clicked one', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/a', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/b', lastOpened: '2026-01-02T00:00:00Z' },
    ])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()

    expect(wrapper.text()).toContain('/a')
    expect(wrapper.text()).toContain('/b')

    await wrapper.get('.p-tree-node-content').trigger('click')

    expect(mockSelectWorktree).toHaveBeenCalledTimes(1)
    expect(mockSelectWorktree).toHaveBeenCalledWith('/a')
  })

  it('FR-REPO-07 shows the WSL distro of a WSL recent repo, not of a local one', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/local', lastOpened: '2026-01-01T00:00:00Z' },
      {
        path: '\\\\wsl.localhost\\Ubuntu-22.04\\home\\user\\project',
        lastOpened: '2026-01-02T00:00:00Z',
        distro: 'Ubuntu-22.04',
      },
    ])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()

    const labels = wrapper.findAll('.repo-node').map((n) => n.text())
    expect(labels).toHaveLength(2)
    expect(labels[0]).not.toContain('Ubuntu-22.04')
    expect(labels[1]).toContain('Ubuntu-22.04')
  })

  it('FR-REPO-07 lists distros in the WSL folder dialog, WSL 1 shown disabled', async () => {
    mockListWSLDistros.mockResolvedValue([
      { name: 'Ubuntu-22.04', version: 2, default: true },
      { name: 'Old Distro', version: 1, default: false },
    ])

    const body = await openWSLDialog()

    expect(body.text()).toContain('Ubuntu-22.04')
    expect(body.text()).toContain('Old Distro')
    expect(body.text()).toContain('WSL 1 is not supported')
    const wsl1Button = body
      .findAll('[data-test="wsl-distro"]')
      .find((b) => b.text().includes('Old Distro'))
    expect(wsl1Button?.attributes('disabled')).toBeDefined()
  })

  it('FR-REPO-07 calls PickAndOpenWSL with the selected distro on Browse', async () => {
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, default: true }])

    const body = await openWSLDialog()
    await body.get('[data-test="wsl-distro"]').trigger('click')
    await body.get('[data-test="wsl-browse"]').trigger('click')
    await flushPromises()

    expect(mockPickAndOpenWSL).toHaveBeenCalledTimes(1)
    expect(mockPickAndOpenWSL).toHaveBeenCalledWith('Ubuntu-22.04')
  })

  it('FR-REPO-07 calls OpenPath with the UNC path built from the selected distro and typed folder', async () => {
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, default: true }])

    const body = await openWSLDialog()
    await body.get('[data-test="wsl-distro"]').trigger('click')
    await body.get('input[placeholder="Folder inside the distro"]').setValue('home/user/project')
    await body.get('[data-test="wsl-open"]').trigger('click')
    await flushPromises()

    expect(mockOpenPath).toHaveBeenCalledTimes(1)
    expect(mockOpenPath).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu-22.04\\home\\user\\project')
  })

  it('FR-WT-13 expanding a repo node lists its worktrees and branches once, not the other repo\'s', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/repoB', lastOpened: '2026-01-02T00:00:00Z' },
    ])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])
    mockListBranches.mockResolvedValue([{ name: 'feature', checkedOutIn: '/repoA/wt' }])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()

    await expandRepo(wrapper, '/repoA')

    expect(mockListWorktrees).toHaveBeenCalledTimes(1)
    expect(mockListWorktrees).toHaveBeenCalledWith('/repoA')
    expect(mockListBranches).toHaveBeenCalledTimes(1)
    expect(mockListBranches).toHaveBeenCalledWith('/repoA')
    expect(wrapper.text()).toContain('feature')
  })

  it('FR-WT-05 clicking a worktree row calls SelectWorktree with its path', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="worktree-row"]').trigger('click')

    expect(mockSelectWorktree).toHaveBeenCalledTimes(1)
    expect(mockSelectWorktree).toHaveBeenCalledWith('/repoA/wt')
  })

  it('FR-WT-08 the open-in-new-window control calls OpenRecent, not SelectWorktree', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="open-in-new-window"]').trigger('click')

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent).toHaveBeenCalledWith('/repoA/wt')
    expect(mockSelectWorktree).not.toHaveBeenCalled()
  })

  it('FR-WT-04 an ordinary worktree row shows "Open in editor", a missing one does not', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/repoA/wt' }),
      worktreeFixture({ path: '/repoA/wt-missing', missing: true }),
    ])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    const rows = wrapper.findAll('[data-test="worktree-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0]!.find('[data-test="open-in-editor"]').exists()).toBe(true)
    expect(rows[1]!.find('[data-test="open-in-editor"]').exists()).toBe(false)
  })

  it('FR-WT-04 the "Open in editor" control calls Facade.OpenFolderAt with the row\'s path', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="open-in-editor"]').trigger('click')

    expect(mockOpenFolderAt).toHaveBeenCalledTimes(1)
    expect(mockOpenFolderAt).toHaveBeenCalledWith('/repoA/wt')
  })

  it('FR-BR-01, FR-BR-05 "New branch…" and per-branch "Checkout" render only under the selected repo\'s node', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/repoB', lastOpened: '2026-01-02T00:00:00Z' },
    ])
    mockListWorktrees.mockImplementation((path: string) =>
      Promise.resolve([worktreeFixture({ path: `${path}/wt` })]),
    )
    mockListBranches.mockResolvedValue([{ name: 'feature', checkedOutIn: '' }])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '/repoA/wt' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')
    await expandRepo(wrapper, '/repoB')

    expect(wrapper.findAll('[data-test="new-branch"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-test="checkout-branch"]')).toHaveLength(1)
  })

  it('FR-WT-13 clicking a branch row with checkedOutIn set calls SelectWorktree with that worktree\'s path', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])
    mockListBranches.mockResolvedValue([{ name: 'feature', checkedOutIn: '/repoA/wt2' }])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="branch-row"]').trigger('click')

    expect(mockSelectWorktree).toHaveBeenCalledTimes(1)
    expect(mockSelectWorktree).toHaveBeenCalledWith('/repoA/wt2')
  })

  it('FR-WT-02 "+ New worktree" opens CreateWorktreeDialog for that repo, regardless of selection', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])
    mockListBranches.mockResolvedValue([{ name: 'feature', checkedOutIn: '' }])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="new-worktree"]').trigger('click')

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('repoPath')).toBe('/repoA')
    expect(dialog.props('branches')).toEqual([{ name: 'feature', checkedOutIn: '' }])
  })

  it('FR-UX-07 the collapse control hides and shows the panel\'s own content, staying visible itself', async () => {
    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' }, attachTo: document.body })
    await flushPromises()

    expect(wrapper.find('.panel-body').isVisible()).toBe(true)

    const collapseButton = wrapper.get('[data-test="panel-collapse"]')
    await collapseButton.trigger('click')

    expect(wrapper.find('.panel-body').isVisible()).toBe(false)
    expect(collapseButton.isVisible()).toBe(true)

    await collapseButton.trigger('click')

    expect(wrapper.find('.panel-body').isVisible()).toBe(true)
    wrapper.unmount()
  })

  it('FR-UX-04 clicking Settings emits open-settings', async () => {
    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()

    await wrapper.get('[data-test="open-settings"]').trigger('click')

    expect(wrapper.emitted('open-settings')).toHaveLength(1)
  })

  it('FR-WT-10 a repo:changed{scope:"worktrees"} event re-fetches only the selected repo\'s worktrees', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/repoB', lastOpened: '2026-01-02T00:00:00Z' },
    ])
    mockListWorktrees.mockImplementation((path: string) =>
      Promise.resolve([worktreeFixture({ path: `${path}/wt` })]),
    )

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '/repoA/wt' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')
    await expandRepo(wrapper, '/repoB')
    mockListWorktrees.mockClear()

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'repo:changed')?.[1]
    handler?.({ data: { scope: 'worktrees' } })
    await flushPromises()

    expect(mockListWorktrees).toHaveBeenCalledTimes(1)
    expect(mockListWorktrees).toHaveBeenCalledWith('/repoA')
  })

  it('FR-UX-01 a rejected PickAndOpen renders ErrorBanner with the error', async () => {
    mockPickAndOpen.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'dialog cancelled' } }),
    )
    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })

    const openFolderButton = wrapper.findAll('button').find((b) => b.text() === 'Open folder…')
    await openFolderButton!.trigger('click')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'dialog cancelled' })
  })
})
