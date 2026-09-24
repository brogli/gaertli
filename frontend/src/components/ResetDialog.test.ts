import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { useToast } from 'openvue/usetoast'

const { Facade } = vi.hoisted(() => ({ Facade: { Reset: vi.fn(), UncommittedCount: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
import { confirmOptions, confirmRequire } from '../testing/confirm'

import ResetDialog from './ResetDialog.vue'
import NoticeArea from './NoticeArea.vue'
import { mountInBody } from '../testing/mountInBody'
import { NOTICE_GROUP } from '../lib/notice'

function mountDialog(branch = 'main') {
  return mountInBody(ResetDialog, {
    props: { wt: '/repo', branch, sha: 'abc123', subject: 'Fix the bug' },
  })
}

// resetTo runs one successful reset whose previous commit is previousHead.
async function resetTo(previousHead: string): Promise<void> {
  Facade.Reset.mockResolvedValueOnce({ previousHead })
  const { body } = await mountDialog()
  await body.get('[data-test="reset-submit"]').trigger('click')
  await flushPromises()
}

describe('ResetDialog', () => {
  beforeEach(() => {
    Facade.Reset.mockReset()
    Facade.UncommittedCount.mockReset().mockResolvedValue(0)
    confirmRequire.mockReset()
  })

  it('FR-BR-02 the first line names the branch, the short hash and the subject', async () => {
    const { body } = await mountDialog()

    expect(body.text()).toContain('Reset main to abc123 Fix the bug')
  })

  it('FR-BR-02 the four modes render one per line, each its own block, mixed preselected', async () => {
    const { body } = await mountDialog()

    const modes = body.findAll('.reset-mode')
    expect(modes.map((m) => m.text())).toEqual([
      "Soft Keep the undone commits' changes staged.",
      "Mixed Keep the undone commits' changes in your files, unstaged.",
      "Hard Discard the undone commits' changes and all uncommitted changes.",
      "Keep Discard the undone commits' changes, keep uncommitted ones. Stops if they touch the same files.",
    ])
    expect((body.get('input#reset-mode-mixed').element as HTMLInputElement).checked).toBe(true)
  })

  it('FR-BR-02 Reset fails: the dialog stays open with the error, no notice', async () => {
    Facade.Reset.mockRejectedValue(new Error('boom'))
    const { wrapper, body: dialog } = await mountDialog()
    const { body: notice } = await mountInBody(NoticeArea)

    await dialog.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(dialog.find('.error-banner').exists()).toBe(true)
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(notice.find('.notice').exists()).toBe(false)
  })

  it('FR-BR-07 clicking Reset at keep mode calls Facade.Reset directly, no confirmation', async () => {
    Facade.Reset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('input#reset-mode-keep').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Reset).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).toHaveBeenCalledWith('/repo', 'abc123', 'keep')
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it('FR-BR-02 renders "HEAD" instead of a blank branch name while detached', async () => {
    const { body } = await mountDialog('')

    expect(body.text()).toContain('HEAD')
  })

  it('FR-BR-02 clicking Reset at the default mixed mode calls Facade.Reset directly, no confirmation', async () => {
    Facade.Reset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Reset).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).toHaveBeenCalledWith('/repo', 'abc123', 'mixed')
    expect(confirmRequire).not.toHaveBeenCalled()
  })

  it('FR-BR-04 a successful reset closes the dialog and its notice names the previous short hash, Copy hash copies the full one', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    Facade.Reset.mockResolvedValue({ previousHead: 'deadbeefcafe' })
    const { wrapper, body: dialog } = await mountDialog()
    const { body: notice } = await mountInBody(NoticeArea)

    await dialog.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(notice.text()).toContain('deadbee') // the previous commit's short hash

    await notice.get('[data-test="notice-copy"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('deadbeefcafe')
  })

  it('FR-BR-04 a second reset replaces the notice with its own', async () => {
    const { body: notice } = await mountInBody(NoticeArea)

    await resetTo('first0000000')
    expect(notice.findAll('.notice')).toHaveLength(1)
    expect(notice.text()).toContain('first00')

    await resetTo('second0000000')

    expect(notice.findAll('.notice')).toHaveLength(1)
    expect(notice.text()).toContain('second0')
    expect(notice.text()).not.toContain('first00')
  })

  it("FR-UX-15 a reset's notice leaves another action's notice open", async () => {
    const { body: notice } = await mountInBody(NoticeArea)
    const OtherAction = defineComponent({
      setup() {
        useToast().add({ group: NOTICE_GROUP, summary: 'Push was rejected', closable: true })
        return () => null
      },
    })
    mount(OtherAction)
    await flushPromises()

    await resetTo('first0000000')
    await resetTo('second0000000')

    expect(notice.findAll('.notice')).toHaveLength(2)
    expect(notice.text()).toContain('Push was rejected')
    expect(notice.text()).toContain('second0')
  })

  it('FR-BR-03 hard mode confirms first, naming the uncommitted count, before calling Facade.Reset', async () => {
    Facade.UncommittedCount.mockResolvedValue(4)
    Facade.Reset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Reset).not.toHaveBeenCalled()
    expect(confirmRequire).toHaveBeenCalledTimes(1)
    expect(confirmOptions().message).toContain('4')

    confirmOptions().accept()
    await flushPromises()

    expect(Facade.Reset).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).toHaveBeenCalledWith('/repo', 'abc123', 'hard')
  })

  it('FR-UX-03 never calls Facade.Reset if the confirmation is not accepted (QA-SAFE-01)', async () => {
    Facade.UncommittedCount.mockResolvedValue(4)
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await body.get('[data-test="reset-submit"]').trigger('click')
    await flushPromises()

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).not.toHaveBeenCalled()
  })
})
