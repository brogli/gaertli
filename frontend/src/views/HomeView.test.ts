import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const {
  mockPickAndOpen,
  mockRecentRepos,
  mockOpenRecent,
  mockListWSLDistros,
  mockPickAndOpenWSL,
  mockOpenPath,
} = vi.hoisted(() => ({
  mockPickAndOpen: vi.fn(),
  mockRecentRepos: vi.fn(),
  mockOpenRecent: vi.fn(),
  mockListWSLDistros: vi.fn(),
  mockPickAndOpenWSL: vi.fn(),
  mockOpenPath: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    PickAndOpen: mockPickAndOpen,
    RecentRepos: mockRecentRepos,
    OpenRecent: mockOpenRecent,
    ListWSLDistros: mockListWSLDistros,
    PickAndOpenWSL: mockPickAndOpenWSL,
    OpenPath: mockOpenPath,
  },
}))

import HomeView from './HomeView.vue'

describe('HomeView', () => {
  let wslWrapper: VueWrapper | undefined

  beforeEach(() => {
    mockPickAndOpen.mockReset().mockResolvedValue(undefined)
    mockRecentRepos.mockReset().mockResolvedValue([])
    mockOpenRecent.mockReset().mockResolvedValue(undefined)
    mockListWSLDistros.mockReset().mockResolvedValue([])
    mockPickAndOpenWSL.mockReset().mockResolvedValue(undefined)
    mockOpenPath.mockReset().mockResolvedValue(undefined)
  })

  afterEach(() => {
    wslWrapper?.unmount()
    wslWrapper = undefined
  })

  // openWSLDialog mounts HomeView, clicks "Open WSL folder…", and hands
  // back a DOMWrapper on the dialog's teleported content (shared by cases
  // 18-20); wslWrapper is unmounted in afterEach.
  async function openWSLDialog(): Promise<DOMWrapper<HTMLElement>> {
    wslWrapper = mount(HomeView, { attachTo: document.body })
    await wslWrapper.get('[data-test="open-wsl-folder"]').trigger('click')
    await flushPromises()
    return new DOMWrapper(document.body)
  }

  it('FR-REPO-01 opens the folder picker on click', async () => {
    const wrapper = mount(HomeView)

    await wrapper.get('button').trigger('click')

    expect(mockPickAndOpen).toHaveBeenCalledTimes(1)
  })

  it('FR-REPO-02 lists recent repos and opens the clicked one', async () => {
    mockRecentRepos.mockResolvedValue([
      { path: '/a', lastOpened: '2026-01-01T00:00:00Z' },
      { path: '/b', lastOpened: '2026-01-02T00:00:00Z' },
    ])

    const wrapper = mount(HomeView)
    await flushPromises()

    expect(wrapper.text()).toContain('/a')
    expect(wrapper.text()).toContain('/b')

    await wrapper.get('li button').trigger('click')

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent).toHaveBeenCalledWith('/a')
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

    const wrapper = mount(HomeView)
    await flushPromises()

    const items = wrapper.findAll('li')
    expect(items).toHaveLength(2)
    expect(items[0]?.text()).not.toContain('Ubuntu-22.04')
    expect(items[1]?.text()).toContain('Ubuntu-22.04')
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
})
