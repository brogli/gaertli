import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { CreateBranch: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import CreateBranchDialog from './CreateBranchDialog.vue'
import { mountInBody } from '../testing/mountInBody'
import { field, fieldLabels, footerButtons } from '../testing/dialog'
import { branchFixture } from '../testing/fixtures/branch'
import { useRefsStore } from '../stores/refs'
import type { StartPoint } from './startPoint'

function mountDialog(start: StartPoint) {
  useRefsStore().refs = {
    branches: [branchFixture(), branchFixture({ name: 'feat', current: false })],
    remotes: [{ name: 'origin', branches: [{ name: 'x', sha: 'sha-x' }] }],
    tags: [{ name: 'v1', sha: 'sha-v1' }],
  }
  return mountInBody(CreateBranchDialog, { props: { wt: '/repo', start } })
}

async function create(body: Awaited<ReturnType<typeof mountDialog>>['body'], name: string) {
  await field(body, 'Branch name').setValue(name)
  await body.get('[data-test="create-branch-submit"]').trigger('click')
  await flushPromises()
}

describe('CreateBranchDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CreateBranch.mockReset().mockResolvedValue(undefined)
  })

  it('FR-BR-01 a fixed start (a commit) shows its label, not a list, and submits the name, the commit and checkout true', async () => {
    const { body } = await mountDialog({ rev: 'abc123456', label: 'abc1234' })

    const start = field(body, 'Start point')
    expect(start.element.value).toBe('abc1234')
    expect(start.element.disabled).toBe(true)
    await create(body, 'feature')

    expect(Facade.CreateBranch).toHaveBeenCalledWith('/repo', 'feature', 'abc123456', true)
  })

  it('FR-BR-01 a start is preselected in a list of the local branches, remote branches and tags; the one picked is sent by its full ref name', async () => {
    const { body } = await mountDialog({ ref: 'refs/heads/feat' })

    expect(body.get('[data-test="start-branch"]').text()).toBe('feat')
    await body.get('[data-test="start-branch"]').trigger('click')
    await flushPromises()
    expect(body.findAll('[role="option"]').map((o) => o.text())).toEqual([
      'Local',
      'main',
      'feat',
      'Remote',
      'origin/x',
      'Tags',
      'v1',
    ])
    await body.get('[role="option"][aria-label="v1"]').trigger('mousedown')
    await create(body, 'feature')

    expect(Facade.CreateBranch).toHaveBeenCalledWith('/repo', 'feature', 'refs/tags/v1', true)
  })

  it('FR-BR-01 shows the rejection message and keeps the dialog open on failure', async () => {
    Facade.CreateBranch.mockRejectedValueOnce(
      new Error('boom', { cause: { message: '"-bad" is not a valid branch name' } }),
    )
    const { wrapper, body } = await mountDialog({ ref: 'refs/heads/main' })

    await create(body, '-bad')

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-UX-17 name and start point each have their own label; Cancel and Create sit together at the bottom', async () => {
    const { wrapper, body } = await mountDialog({ ref: 'refs/heads/main' })

    expect(fieldLabels(body)).toEqual(['Branch name', 'Start point'])
    expect(footerButtons(body)).toEqual(['Cancel', 'Create'])
    await body.get('[data-test="dialog-cancel"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
