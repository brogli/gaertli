// Requirements: FR-SYNC-11, FR-SYNC-12
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Push: vi.fn() } }))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import SetUpstreamDialog from '../SetUpstreamDialog.vue'
import { mountInBody } from '@/testing/mountInBody'
import { useRefsStore } from '@/stores/refs'

function mountDialog(remotes: string[], remote: string, branch = 'feature') {
  useRefsStore().refs = {
    branches: [],
    remotes: remotes.map((name) => ({ name, branches: [] })),
    tags: [],
  }
  return mountInBody(SetUpstreamDialog, { props: { wt: '/repo', branch, remote } })
}

describe('SetUpstreamDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Push.mockReset().mockResolvedValue('job-1')
  })

  it('one remote: names it, no select; confirm pushes to it', async () => {
    const { body } = await mountDialog(['origin'], 'origin')

    expect(body.text()).toContain('origin/feature')
    expect(body.find('[data-test="set-upstream-remote"]').exists()).toBe(false)

    await body.get('[data-test="set-upstream-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', 'feature')
  })

  it('cancel closes without pushing', async () => {
    const { wrapper, body } = await mountDialog(['origin'], 'origin')

    await body.get('[data-test="dialog-cancel"]').trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(Facade.Push).not.toHaveBeenCalled()
  })

  it('several remotes, nothing preselected: select shown, confirm disabled until picked, then pushes to it', async () => {
    const { body } = await mountDialog(['up', 'other'], '')

    const select = body.get('[data-test="set-upstream-remote"]')
    expect(select.text()).toBe('')
    expect(body.get<HTMLButtonElement>('[data-test="set-upstream-confirm"]').element.disabled).toBe(
      true,
    )

    await select.trigger('click')
    await flushPromises()
    await body.get('[role="option"][aria-label="other"]').trigger('mousedown')

    expect(body.get<HTMLButtonElement>('[data-test="set-upstream-confirm"]').element.disabled).toBe(
      false,
    )
    await body.get('[data-test="set-upstream-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'other', 'feature')
  })

  it('several remotes, one preselected: select shows it', async () => {
    const { body } = await mountDialog(['up', 'origin'], 'origin')

    expect(body.get('[data-test="set-upstream-remote"]').text()).toBe('origin')
  })
})
