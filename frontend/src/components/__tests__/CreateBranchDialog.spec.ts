// Requirements: FR-BR-01
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { CreateBranch: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import CreateBranchDialog from '../CreateBranchDialog.vue'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { field } from '@/testing/dialog'
import { branchFixture } from '@/testing/fixtures/branch'
import { refsOfEveryKind } from '@/testing/fixtures/refs'
import { useRefsStore } from '@/stores/refs'
import type { StartPoint } from '../startPoint'

function mountDialog(start: StartPoint) {
  useRefsStore().refs = refsOfEveryKind([
    branchFixture(),
    branchFixture({ name: 'feat', current: false }),
  ])
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

  it('a fixed start (a commit) shows its label, not a list, and submits the name, the commit and checkout true', async () => {
    const { body } = await mountDialog({ rev: 'abc123456', label: 'abc1234' })

    const start = field(body, 'Start point')
    expect(start.element.value).toBe('abc1234')
    expect(start.element.disabled).toBe(true)
    await create(body, 'feature')

    expect(Facade.CreateBranch).toHaveBeenCalledWith('/repo', 'feature', 'abc123456', true)
  })

  it('unchecking "Check out after creation" submits checkout false', async () => {
    const { body } = await mountDialog({ rev: 'abc123456', label: 'abc1234' })

    await body.get('input[type=checkbox]').setValue(false)
    await create(body, 'later')

    expect(Facade.CreateBranch).toHaveBeenCalledWith('/repo', 'later', 'abc123456', false)
  })

  it('a start is preselected in a list of the local branches, remote branches and tags; the one picked is sent by its full ref name', async () => {
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

  it('shows the rejection message and keeps the dialog open on failure', async () => {
    Facade.CreateBranch.mockRejectedValueOnce(wailsError('"-bad" is not a valid branch name'))
    const { wrapper, body } = await mountDialog({ ref: 'refs/heads/main' })

    await create(body, '-bad')

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
