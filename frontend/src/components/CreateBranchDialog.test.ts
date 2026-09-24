import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { CreateBranch: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import CreateBranchDialog from './CreateBranchDialog.vue'
import { mountInBody } from '../testing/mountInBody'

describe('CreateBranchDialog', () => {
  beforeEach(() => {
    Facade.CreateBranch.mockReset()
  })

  it('FR-BR-01 submits the name, sha, and checkout defaulting true', async () => {
    Facade.CreateBranch.mockResolvedValue(undefined)
    const { body } = await mountInBody(CreateBranchDialog, {
      props: { wt: '/repo', sha: 'abc123' },
    })

    await body.get('input[placeholder="Branch name"]').setValue('feature')
    await body.get('[data-test="create-branch-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.CreateBranch).toHaveBeenCalledTimes(1)
    expect(Facade.CreateBranch).toHaveBeenCalledWith('/repo', 'feature', 'abc123', true)
  })

  it('FR-BR-01 shows the rejection message and keeps the dialog open on failure', async () => {
    Facade.CreateBranch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: '"-bad" is not a valid branch name' } }),
    )
    const { wrapper, body } = await mountInBody(CreateBranchDialog, {
      props: { wt: '/repo', sha: 'abc123' },
    })

    await body.get('input[placeholder="Branch name"]').setValue('-bad')
    await body.get('[data-test="create-branch-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
