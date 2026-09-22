import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { mockJumpTo, mockCheckout, mockUpdateBranch } = vi.hoisted(() => ({
  mockJumpTo: vi.fn(),
  mockCheckout: vi.fn(),
  mockUpdateBranch: vi.fn(),
}))

const mockRefs = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => ({ refs: mockRefs() }),
}))

vi.mock('../stores/log', () => ({
  useLogStore: () => ({ jumpTo: mockJumpTo }),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Checkout: mockCheckout, UpdateBranch: mockUpdateBranch },
}))

import RefsSidebar from './RefsSidebar.vue'
import ErrorBanner from './ErrorBanner.vue'
import { useJobsStore } from '../stores/jobs'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '../testing/fixtures/branch'
import { clickMenuItem } from '../testing/contextMenu'

function updateJob(fields: Partial<JobInfo>): JobInfo {
  return {
    id: '1',
    kind: 'update-branch:feature',
    state: 'running',
    progress: -1,
    message: '',
    startedAt: '2024-01-01T00:00:00Z',
    ...fields,
  }
}

function treeItem(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper
    .findAll('[role="treeitem"]')
    .find((node) => node.attributes('aria-label') === label)
}

async function openContextMenu(wrapper: ReturnType<typeof mount>, label: string) {
  const item = treeItem(wrapper, label)
  await item?.find('.ref-node').trigger('contextmenu')
  await wrapper.vm.$nextTick()
}

describe('RefsSidebar', () => {
  beforeEach(() => {
    mockJumpTo.mockReset()
    mockCheckout.mockReset()
    mockCheckout.mockResolvedValue(undefined)
    mockUpdateBranch.mockReset()
    mockUpdateBranch.mockResolvedValue(undefined)
    setActivePinia(createPinia())
    mockRefs.mockReset()
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(),
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false }),
      ],
      remotes: [],
      tags: [],
    })
  })

  it('FR-LOG-01 renders empty sections with no crash when a repo has no refs at all', () => {
    mockRefs.mockReturnValue({ branches: [], remotes: [], tags: [] })

    expect(() => mount(RefsSidebar, { props: { wt: '/repo' } })).not.toThrow()
  })

  it('FR-LOG-01 marks the current branch and not the other', () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })

    expect(treeItem(wrapper, 'main')?.find('[data-test="current-marker"]').exists()).toBe(true)
    expect(treeItem(wrapper, 'feature')?.find('[data-test="current-marker"]').exists()).toBe(false)
  })

  it('FR-LOG-01 clicking a branch leaf calls jumpTo with its sha', async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })

    await treeItem(wrapper, 'main')?.find('.p-tree-node-content').trigger('click')

    expect(mockJumpTo).toHaveBeenCalledTimes(1)
    expect(mockJumpTo).toHaveBeenCalledWith('sha-main')
  })

  it('FR-BR-05 right-clicking a non-current branch offers Checkout, calling Facade.Checkout with its name', async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual(['Checkout', 'Create worktree here…'])

    await clickMenuItem(body, 'Checkout')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it("FR-BR-05 the current branch's own menu has no Checkout item", async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'main')

    const body = new DOMWrapper(document.body)
    expect(body.findAll('.p-contextmenu-item-label')).toHaveLength(0)
    expect(mockCheckout).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-BR-05 checking out a remote branch uses its short name, no remote prefix', async () => {
    mockRefs.mockReturnValue({
      branches: [],
      remotes: [{ name: 'origin', branches: [{ name: 'feature', sha: 'sha-feature' }] }],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')

    const body = new DOMWrapper(document.body)
    await clickMenuItem(body, 'Checkout')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it('FR-WT-06 "Create worktree here…" on a local branch emits create-worktree with isRemote false', async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')
    const body = new DOMWrapper(document.body)

    await clickMenuItem(body, 'Create worktree here…')

    expect(wrapper.emitted('create-worktree')).toEqual([['feature', false]])

    wrapper.unmount()
  })

  it('FR-WT-06 "Create worktree here…" on a remote branch emits create-worktree with isRemote true', async () => {
    mockRefs.mockReturnValue({
      branches: [],
      remotes: [{ name: 'origin', branches: [{ name: 'feature', sha: 'sha-feature' }] }],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')
    const body = new DOMWrapper(document.body)

    await clickMenuItem(body, 'Create worktree here…')

    expect(wrapper.emitted('create-worktree')).toEqual([['feature', true]])

    wrapper.unmount()
  })

  it('FR-UPD-01 right-clicking a non-current local branch with an upstream offers "Update from upstream", calling Facade.UpdateBranch', async () => {
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(),
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false, upstream: 'origin/feature' }),
      ],
      remotes: [],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual(['Checkout', 'Create worktree here…', 'Update from upstream'])

    await clickMenuItem(body, 'Update from upstream')
    await flushPromises()

    expect(mockUpdateBranch).toHaveBeenCalledTimes(1)
    expect(mockUpdateBranch).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it('FR-UPD-05 a done update-branch job renders a result message naming the branch and commit count', async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })
    useJobsStore().upsert(
      updateJob({
        state: 'done',
        result: { outcome: 'updated', from: 'aaa', to: 'bbbbbbbcccc', commits: 3 },
      }),
    )
    await flushPromises()

    const banner = wrapper.get('[data-test="update-result"]')
    expect(banner.text()).toContain('feature')
    expect(banner.text()).toContain('3')
    expect(wrapper.findComponent(ErrorBanner).exists()).toBe(false)
  })

  it('FR-UPD-05 a failed update-branch job renders ErrorBanner with the job\'s error', async () => {
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })
    useJobsStore().upsert(
      updateJob({ state: 'failed', error: { code: 'NOT_FAST_FORWARD', message: 'diverged' } } as Partial<JobInfo>),
    )
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ code: 'NOT_FAST_FORWARD', message: 'diverged' })
  })

  it('FR-UX-01 a rejected UpdateBranch call renders ErrorBanner with the error, no job started', async () => {
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(),
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false, upstream: 'origin/feature' }),
      ],
      remotes: [],
      tags: [],
    })
    mockUpdateBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'no upstream configured', code: 'NO_UPSTREAM' } }),
    )
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' }, attachTo: document.body })
    await openContextMenu(wrapper, 'feature')
    const body = new DOMWrapper(document.body)

    await clickMenuItem(body, 'Update from upstream')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'no upstream configured', code: 'NO_UPSTREAM' })

    wrapper.unmount()
  })
})
