import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCreateBranch } = vi.hoisted(() => ({ mockCreateBranch: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CreateBranch: mockCreateBranch },
}))

import CreateBranchDialog from './CreateBranchDialog.vue'

describe('CreateBranchDialog', () => {
  let wrapper: VueWrapper | undefined

  beforeEach(() => {
    mockCreateBranch.mockReset()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('FR-BR-01 submits the name, sha, and checkout defaulting true', async () => {
    mockCreateBranch.mockResolvedValue(undefined)
    wrapper = mount(CreateBranchDialog, {
      props: { wt: '/repo', sha: 'abc123', visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('input[placeholder="Branch name"]').setValue('feature')
    await body.get('[data-test="create-branch-submit"]').trigger('click')
    await flushPromises()

    expect(mockCreateBranch).toHaveBeenCalledTimes(1)
    expect(mockCreateBranch).toHaveBeenCalledWith('/repo', 'feature', 'abc123', true)
  })

  it('FR-BR-01 shows the rejection message and keeps the dialog open on failure', async () => {
    mockCreateBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: '"-bad" is not a valid branch name' } }),
    )
    wrapper = mount(CreateBranchDialog, {
      props: { wt: '/repo', sha: 'abc123', visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('input[placeholder="Branch name"]').setValue('-bad')
    await body.get('[data-test="create-branch-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('update:visible')).toBeUndefined()
  })
})
