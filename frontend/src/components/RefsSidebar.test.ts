import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mountInBody } from '../testing/mountInBody'

const { mockJumpTo, mockCheckout, mockUpdateBranch, mockRenameBranch, mockDeleteBranch, mockRequire } =
  vi.hoisted(() => ({
    mockJumpTo: vi.fn(),
    mockCheckout: vi.fn(),
    mockUpdateBranch: vi.fn(),
    mockRenameBranch: vi.fn(),
    mockDeleteBranch: vi.fn(),
    mockRequire: vi.fn(),
  }))

const mockRefs = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => ({ refs: mockRefs() }),
}))

vi.mock('../stores/log', () => ({
  useLogStore: () => ({ jumpTo: mockJumpTo }),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    Checkout: mockCheckout,
    UpdateBranch: mockUpdateBranch,
    RenameBranch: mockRenameBranch,
    DeleteBranch: mockDeleteBranch,
  },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

import RefsSidebar from './RefsSidebar.vue'
import ErrorBanner from './ErrorBanner.vue'
import RenameBranchDialog from './RenameBranchDialog.vue'
import { useJobsStore } from '../stores/jobs'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '../testing/fixtures/branch'
import { clickMenuItem } from '../testing/contextMenu'

function updateJob(fields: Partial<JobInfo>): JobInfo {
  return {
    id: '1',
    kind: 'update-branch',
    branch: 'feature',
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
    mockRenameBranch.mockReset()
    mockRenameBranch.mockResolvedValue(undefined)
    mockDeleteBranch.mockReset()
    mockDeleteBranch.mockResolvedValue(undefined)
    mockRequire.mockReset()
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
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual(['Checkout', 'Create worktree here…', 'Rename…', 'Delete'])

    await clickMenuItem(body, 'Checkout')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it("FR-BR-05 the current branch's own menu has no Checkout item", async () => {
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'main')

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
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Checkout')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it('FR-BR-06 "Rename…" opens RenameBranchDialog prefilled with the branch\'s name', async () => {
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Rename…')
    await flushPromises()

    expect(wrapper.findComponent(RenameBranchDialog).props()).toMatchObject({
      wt: '/repo',
      name: 'feature',
    })

    wrapper.unmount()
  })

  it('FR-BR-06 "Delete" on a mergeable branch calls Facade.DeleteBranch(wt, name, false) with no confirmation', async () => {
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Delete')
    await flushPromises()

    expect(mockDeleteBranch).toHaveBeenCalledTimes(1)
    expect(mockDeleteBranch).toHaveBeenCalledWith('/repo', 'feature', false)
    expect(mockRequire).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-BR-06, FR-UX-03, QA-SAFE-01 "Delete" on an unmerged branch confirms, naming the reason, then retries forced', async () => {
    mockDeleteBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { code: 'UNMERGED_BRANCH', message: 'not fully merged' },
      }),
    )
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Delete')
    await flushPromises()

    expect(mockDeleteBranch).toHaveBeenCalledTimes(1)
    expect(mockDeleteBranch).toHaveBeenCalledWith('/repo', 'feature', false)
    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
    expect(options.message).toContain('not fully merged')
    expect(wrapper.findComponent(ErrorBanner).exists()).toBe(false)

    options.accept()
    await flushPromises()

    expect(mockDeleteBranch).toHaveBeenCalledTimes(2)
    expect(mockDeleteBranch).toHaveBeenLastCalledWith('/repo', 'feature', true)

    wrapper.unmount()
  })

  it('FR-BR-06 "Delete" refused for another reason shows ErrorBanner, no confirmation', async () => {
    mockDeleteBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { code: 'GIT_FAILED', message: 'used by worktree' } }),
    )
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Delete')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ code: 'GIT_FAILED', message: 'used by worktree' })
    expect(mockRequire).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-WT-06 "Create worktree here…" on a local branch emits create-worktree with isRemote false', async () => {
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

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
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

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
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual([
      'Checkout',
      'Create worktree here…',
      'Update from upstream',
      'Rename…',
      'Delete',
    ])

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
    const { wrapper, body } = await mountInBody(RefsSidebar, { props: { wt: '/repo' } })
    await openContextMenu(wrapper, 'feature')

    await clickMenuItem(body, 'Update from upstream')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'no upstream configured', code: 'NO_UPSTREAM' })

    wrapper.unmount()
  })

  it('FR-UPD-06 checking two eligible branches then clicking "Update 2 branches" calls Facade.UpdateBranch once per branch', async () => {
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(),
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false, upstream: 'origin/feature' }),
        branchFixture({ name: 'other', sha: 'sha-other', current: false, upstream: 'origin/other' }),
      ],
      remotes: [],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })

    const checkboxes = wrapper.findAll('[data-test="update-select"]')
    expect(checkboxes).toHaveLength(2) // "main" is current, excluded; "feature" and "other" are eligible
    await checkboxes[0]!.setValue(true)
    await checkboxes[1]!.setValue(true)

    const updateButton = wrapper.findAll('button').find((b) => b.text() === 'Update 2 branches')
    expect(updateButton).toBeTruthy()
    await updateButton!.trigger('click')
    await flushPromises()

    expect(mockUpdateBranch).toHaveBeenCalledTimes(2)
    expect(mockUpdateBranch).toHaveBeenCalledWith('/repo', 'feature')
    expect(mockUpdateBranch).toHaveBeenCalledWith('/repo', 'other')
  })

  it('FR-UPD-06 the current branch and a branch with no upstream offer no selection checkbox', () => {
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(), // current, no upstream
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false, upstream: '' }),
      ],
      remotes: [],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })

    expect(wrapper.findAll('[data-test="update-select"]')).toHaveLength(0)
  })

  it('FR-UX-09 an eligible branch\'s update checkbox has a title and aria-label naming it', () => {
    mockRefs.mockReturnValue({
      branches: [
        branchFixture(),
        branchFixture({ name: 'feature', sha: 'sha-feature', current: false, upstream: 'origin/feature' }),
      ],
      remotes: [],
      tags: [],
    })
    const wrapper = mount(RefsSidebar, { props: { wt: '/repo' } })

    const checkbox = wrapper.get('[data-test="update-select"]')
    expect(checkbox.attributes('title')).toBe('Select feature to update')
    expect(checkbox.attributes('aria-label')).toBe('Select feature to update')
  })
})
