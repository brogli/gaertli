import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

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
  mockRemoveWorktree,
  mockPruneWorktrees,
  mockRequire,
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
  mockRemoveWorktree: vi.fn(),
  mockPruneWorktrees: vi.fn(),
  mockRequire: vi.fn(),
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
    RemoveWorktree: mockRemoveWorktree,
    PruneWorktrees: mockPruneWorktrees,
  },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

const { mockEventsOn } = vi.hoisted(() => ({ mockEventsOn: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Events: { On: mockEventsOn } }))

import SelectorPanel from './SelectorPanel.vue'
import CreateWorktreeDialog from './CreateWorktreeDialog.vue'
import ErrorBanner from './ErrorBanner.vue'
import { worktreeFixture } from '../testing/fixtures/worktree'
import { registeredHandler } from '../testing/events'

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
    mockRemoveWorktree.mockReset().mockResolvedValue(undefined)
    mockPruneWorktrees.mockReset().mockResolvedValue(undefined)
    mockRequire.mockReset()
    mockEventsOn.mockReset().mockReturnValue(() => {})
  })

  afterEach(() => {
    wslWrapper?.unmount()
    wslWrapper = undefined
  })

  // openWSLDialog mounts SelectorPanel, clicks "Open WSL folder…", and
  // hands back a DOMWrapper on the dialog's teleported content; wslWrapper
  // is unmounted in afterEach.
  async function openWSLDialog(): Promise<DOMWrapper<Element>> {
    const { wrapper, body } = await mountInBody(SelectorPanel, { props: { selectedPath: '' } })
    wslWrapper = wrapper
    await wrapper.get('[data-test="open-wsl-folder"]').trigger('click')
    await flushPromises()
    return body
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

    const rows = wrapper.findAll('.repo-node')
    expect(rows.map((r) => r.attributes('title'))).toEqual(['/a', '/b'])

    await wrapper.get('.p-tree-node-content').trigger('click')

    expect(mockSelectWorktree).toHaveBeenCalledTimes(1)
    expect(mockSelectWorktree).toHaveBeenCalledWith('/a')
  })

  it('FR-UX-09 a repo row shows the folder name, the muted parent path, and the full path in title', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/a/b/repo', lastOpened: '2026-01-01T00:00:00Z' }])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()

    const row = wrapper.get('.repo-node')
    expect(row.attributes('title')).toBe('/a/b/repo')
    expect(row.get('.repo-name').text()).toBe('repo')
    expect(row.get('.repo-parent').text()).toBe('/a/b')
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

  it('FR-WT-01 a worktree row shows its folder name after the branch, the full path in title', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture({ path: '/repoA/wt-feature', branch: 'feature' })])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    const row = wrapper.get('[data-test="worktree-row"]')
    const folder = row.get('[data-test="worktree-folder"]')
    expect(folder.text()).toBe('wt-feature')
    expect(folder.attributes('title')).toBe('/repoA/wt-feature')
    expect(row.text().indexOf('feature')).toBeLessThan(row.text().indexOf('wt-feature'))
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

  it('FR-WT-07 "Remove" is hidden for the shown worktree, confirms with the loss named only for the dirty row', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/repoA/wt-shown' }),
      worktreeFixture({ path: '/repoA/wt-clean' }),
      worktreeFixture({ path: '/repoA/wt-dirty', dirty: true }),
    ])

    const { wrapper } = await mountInBody(SelectorPanel, { props: { selectedPath: '/repoA/wt-shown' } })
    await expandRepo(wrapper, '/repoA')

    const rows = wrapper.findAll('[data-test="worktree-row"]')
    expect(rows).toHaveLength(3)
    expect(rows[0]!.find('[data-test="remove-worktree"]').exists()).toBe(false)
    expect(rows[1]!.find('[data-test="remove-worktree"]').exists()).toBe(true)
    expect(rows[2]!.find('[data-test="remove-worktree"]').exists()).toBe(true)

    await rows[1]!.get('[data-test="remove-worktree"]').trigger('click')
    expect(mockRequire).toHaveBeenCalledTimes(1)
    expect((mockRequire.mock.calls[0]![0] as { message: string }).message).not.toContain('uncommitted changes')

    await rows[2]!.get('[data-test="remove-worktree"]').trigger('click')
    expect(mockRequire).toHaveBeenCalledTimes(2)
    expect((mockRequire.mock.calls[1]![0] as { message: string }).message).toContain('uncommitted changes')

    wrapper.unmount()
  })

  it('FR-WT-07 a dirty row removes forced on confirm; a clean row retries forced only after a DIRTY_WORKTREE refusal', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/repoA/wt-clean' }),
      worktreeFixture({ path: '/repoA/wt-dirty', dirty: true }),
    ])
    mockRemoveWorktree.mockImplementation((_repo: string, target: string) => {
      if (target === '/repoA/wt-clean') {
        return Promise.reject(Object.assign(new Error('boom'), { cause: { code: 'DIRTY_WORKTREE', message: 'dirty' } }))
      }
      return Promise.resolve(undefined)
    })

    const { wrapper } = await mountInBody(SelectorPanel, { props: { selectedPath: '' } })
    await expandRepo(wrapper, '/repoA')
    const rows = wrapper.findAll('[data-test="worktree-row"]')

    await rows[0]!.get('[data-test="remove-worktree"]').trigger('click')
    const cleanAccept = mockRequire.mock.calls[0]![0] as { accept: () => void }
    cleanAccept.accept()
    await flushPromises()

    expect(mockRemoveWorktree).toHaveBeenCalledTimes(1)
    expect(mockRemoveWorktree).toHaveBeenCalledWith('/repoA', '/repoA/wt-clean', false)
    expect(wrapper.findComponent(ErrorBanner).exists()).toBe(false)
    expect(mockRequire).toHaveBeenCalledTimes(2)
    const retryAccept = mockRequire.mock.calls[1]![0] as { message: string; accept: () => void }
    expect(retryAccept.message).toContain('uncommitted changes')
    retryAccept.accept()
    await flushPromises()
    expect(mockRemoveWorktree).toHaveBeenCalledTimes(2)
    expect(mockRemoveWorktree).toHaveBeenLastCalledWith('/repoA', '/repoA/wt-clean', true)

    await rows[1]!.get('[data-test="remove-worktree"]').trigger('click')
    const dirtyAccept = mockRequire.mock.calls[2]![0] as { accept: () => void }
    dirtyAccept.accept()
    await flushPromises()

    expect(mockRemoveWorktree).toHaveBeenCalledTimes(3)
    expect(mockRemoveWorktree).toHaveBeenLastCalledWith('/repoA', '/repoA/wt-dirty', true)

    wrapper.unmount()
  })

  it('FR-WT-07 "Clean up stale entries" is shown only for a repo node with a missing worktree', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/repoB', lastOpened: '2026-01-02T00:00:00Z' },
    ])
    mockListWorktrees.mockImplementation((path: string) =>
      Promise.resolve([
        worktreeFixture({ path: `${path}/wt` }),
        ...(path === '/repoA' ? [worktreeFixture({ path: `${path}/wt-missing`, missing: true })] : []),
      ]),
    )

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')
    await expandRepo(wrapper, '/repoB')

    expect(wrapper.findAll('[data-test="prune-worktrees"]')).toHaveLength(1)
  })

  it('FR-WT-07 "Clean up stale entries" calls Facade.PruneWorktrees with the repo\'s path, no confirmation', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/repoA/wt' }),
      worktreeFixture({ path: '/repoA/wt-missing', missing: true }),
    ])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')

    await wrapper.get('[data-test="prune-worktrees"]').trigger('click')
    await flushPromises()

    expect(mockPruneWorktrees).toHaveBeenCalledTimes(1)
    expect(mockPruneWorktrees).toHaveBeenCalledWith('/repoA')
    expect(mockRequire).not.toHaveBeenCalled()
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

  it('FR-WT-13 a WSL recent repo\'s calls all take the UNC path: expand, select, open in new window/editor, remove, prune', async () => {
    const repoUNC = '\\\\wsl.localhost\\Ubuntu\\home\\u\\r'
    const wtUNC = '\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt'
    mockRecentRepos.mockResolvedValue([{ path: '/home/u/r', lastOpened: '2026-01-01T00:00:00Z', distro: 'Ubuntu' }])
    mockListWorktrees.mockResolvedValue([
      worktreeFixture({ path: '/home/u/r/wt' }),
      worktreeFixture({ path: '/home/u/r/wt-missing', missing: true }),
    ])
    mockRemoveWorktree.mockResolvedValue(undefined)

    const { wrapper } = await mountInBody(SelectorPanel, { props: { selectedPath: '' } })
    await expandRepo(wrapper, '/home/u/r (Ubuntu)')

    expect(mockListWorktrees).toHaveBeenCalledWith(repoUNC)
    expect(mockListBranches).toHaveBeenCalledWith(repoUNC)

    await repoTreeItem(wrapper, '/home/u/r (Ubuntu)')!.get('.p-tree-node-content').trigger('click')
    expect(mockSelectWorktree).toHaveBeenCalledWith(repoUNC)

    const row = wrapper.findAll('[data-test="worktree-row"]')[0]!
    await row.trigger('click')
    expect(mockSelectWorktree).toHaveBeenCalledWith(wtUNC)

    await row.get('[data-test="open-in-new-window"]').trigger('click')
    expect(mockOpenRecent).toHaveBeenCalledWith(wtUNC)

    await row.get('[data-test="open-in-editor"]').trigger('click')
    expect(mockOpenFolderAt).toHaveBeenCalledWith(wtUNC)

    await row.get('[data-test="remove-worktree"]').trigger('click')
    const accept = mockRequire.mock.calls[0]![0] as { accept: () => void }
    accept.accept()
    await flushPromises()
    expect(mockRemoveWorktree).toHaveBeenCalledWith(repoUNC, wtUNC, false)

    await wrapper.get('[data-test="prune-worktrees"]').trigger('click')
    await flushPromises()
    expect(mockPruneWorktrees).toHaveBeenCalledWith(repoUNC)

    wrapper.unmount()
  })

  it('FR-WT-13 "+ New worktree" for a WSL repo opens CreateWorktreeDialog with its repo path and distro', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/home/u/r', lastOpened: '2026-01-01T00:00:00Z', distro: 'Ubuntu' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture({ path: '/home/u/r/wt' })])

    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/home/u/r (Ubuntu)')

    await wrapper.get('[data-test="new-worktree"]').trigger('click')

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.props('repoPath')).toBe('/home/u/r')
    expect(dialog.props('distro')).toBe('Ubuntu')
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

  it('FR-WT-02 after a worktree is created the expanded repo lists it, with no watcher event needed', async () => {
    mockRecentRepos.mockResolvedValue([{ path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' }])
    mockListWorktrees.mockResolvedValue([worktreeFixture()])
    mockListBranches.mockResolvedValue([])
    const wrapper = mount(SelectorPanel, { props: { selectedPath: '' } })
    await flushPromises()
    await expandRepo(wrapper, '/repoA')
    await wrapper.get('[data-test="new-worktree"]').trigger('click')

    mockListWorktrees.mockResolvedValue([worktreeFixture(), worktreeFixture({ path: '/repoA-new', branch: 'fresh' })])
    wrapper.findComponent(CreateWorktreeDialog).vm.$emit('created')
    await flushPromises()

    expect(wrapper.text()).toContain('fresh')
    expect(wrapper.find('[data-test="new-worktree"]').exists()).toBe(true)
  })

  it('FR-UX-07 the collapse control hides and shows the panel\'s own content, staying visible itself', async () => {
    const { wrapper } = await mountInBody(SelectorPanel, { props: { selectedPath: '' } })

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

    const handler = registeredHandler(mockEventsOn, 'repo:changed')
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
