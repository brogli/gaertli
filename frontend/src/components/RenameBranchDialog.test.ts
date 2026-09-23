import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockRenameBranch } = vi.hoisted(() => ({ mockRenameBranch: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { RenameBranch: mockRenameBranch },
}))

import RenameBranchDialog from './RenameBranchDialog.vue'
import { mountInBody } from '../testing/mountInBody'

describe('RenameBranchDialog', () => {
  beforeEach(() => {
    mockRenameBranch.mockReset()
  })

  it('FR-BR-06 pre-fills the input with the current name and submits the edited name', async () => {
    mockRenameBranch.mockResolvedValue(undefined)
    const { body } = await mountInBody(RenameBranchDialog, {
      props: { wt: '/repo', name: 'feature' },
    })

    const input = body.get('input').element as HTMLInputElement
    expect(input.value).toBe('feature')

    await body.get('input').setValue('renamed')
    await body.get('[data-test="rename-branch-submit"]').trigger('click')
    await flushPromises()

    expect(mockRenameBranch).toHaveBeenCalledTimes(1)
    expect(mockRenameBranch).toHaveBeenCalledWith('/repo', 'feature', 'renamed')
  })

  it('FR-BR-06 shows the rejection message and keeps the dialog open on failure', async () => {
    mockRenameBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: '"-bad" is not a valid branch name' } }),
    )
    const { wrapper, body } = await mountInBody(RenameBranchDialog, {
      props: { wt: '/repo', name: 'feature' },
    })

    await body.get('input').setValue('-bad')
    await body.get('[data-test="rename-branch-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
