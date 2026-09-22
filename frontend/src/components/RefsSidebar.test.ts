import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockJumpTo, mockCheckout } = vi.hoisted(() => ({
  mockJumpTo: vi.fn(),
  mockCheckout: vi.fn(),
}))

const mockRefs = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => ({ refs: mockRefs() }),
}))

vi.mock('../stores/log', () => ({
  useLogStore: () => ({ jumpTo: mockJumpTo }),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Checkout: mockCheckout },
}))

import RefsSidebar from './RefsSidebar.vue'

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
    mockRefs.mockReset()
    mockRefs.mockReturnValue({
      branches: [
        { name: 'main', sha: 'sha-main', current: true, ahead: 0, behind: 0, upstreamGone: false },
        {
          name: 'feature',
          sha: 'sha-feature',
          current: false,
          ahead: 0,
          behind: 0,
          upstreamGone: false,
        },
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
    expect(labels).toEqual(['Checkout'])

    await body.get('.p-contextmenu-item-content').trigger('click')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })

  it('FR-BR-05 the current branch\'s own menu has no Checkout item', async () => {
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
    await body.get('.p-contextmenu-item-content').trigger('click')
    await flushPromises()

    expect(mockCheckout).toHaveBeenCalledTimes(1)
    expect(mockCheckout).toHaveBeenCalledWith('/repo', 'feature')

    wrapper.unmount()
  })
})
