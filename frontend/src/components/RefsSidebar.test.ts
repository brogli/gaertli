import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockJumpTo } = vi.hoisted(() => ({ mockJumpTo: vi.fn() }))

const mockRefs = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => ({ refs: mockRefs() }),
}))

vi.mock('../stores/log', () => ({
  useLogStore: () => ({ jumpTo: mockJumpTo }),
}))

import RefsSidebar from './RefsSidebar.vue'

function treeItem(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper
    .findAll('[role="treeitem"]')
    .find((node) => node.attributes('aria-label') === label)
}

describe('RefsSidebar', () => {
  beforeEach(() => {
    mockJumpTo.mockReset()
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

    expect(() => mount(RefsSidebar)).not.toThrow()
  })

  it('FR-LOG-01 marks the current branch and not the other', () => {
    const wrapper = mount(RefsSidebar)

    expect(treeItem(wrapper, 'main')?.find('[data-test="current-marker"]').exists()).toBe(true)
    expect(treeItem(wrapper, 'feature')?.find('[data-test="current-marker"]').exists()).toBe(false)
  })

  it('FR-LOG-01 clicking a branch leaf calls jumpTo with its sha', async () => {
    const wrapper = mount(RefsSidebar)

    await treeItem(wrapper, 'main')?.find('.p-tree-node-content').trigger('click')

    expect(mockJumpTo).toHaveBeenCalledTimes(1)
    expect(mockJumpTo).toHaveBeenCalledWith('sha-main')
  })
})
