// Requirements: FR-BR-02, FR-BR-03, FR-BR-04, FR-UX-03
import { flushPromises, mount, type DOMWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { useToast } from 'openvue/usetoast'

const { Facade } = vi.hoisted(() => ({ Facade: { Reset: vi.fn(), UncommittedCount: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ Facade }))

vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { mountNotices } from '@/testing/mountNotices'
import { confirmOptions, confirmRequire } from '@/testing/confirm'

import ResetDialog from '../ResetDialog.vue'
import { mountInBody } from '@/testing/mountInBody'
import { NOTICE_GROUP } from '@/lib/notice'

function mountDialog(branch = 'main') {
  return mountInBody(ResetDialog, {
    props: { wt: '/repo', branch, sha: 'abc123', subject: 'Fix the bug' },
  })
}

async function submit(dialog: DOMWrapper<Element>): Promise<void> {
  await dialog.get('[data-test="reset-submit"]').trigger('click')
  await flushPromises()
}

// resetTo runs one successful reset whose previous commit is previousHead.
async function resetTo(previousHead: string): Promise<void> {
  Facade.Reset.mockResolvedValueOnce({ previousHead })
  await submit((await mountDialog()).body)
}

describe('ResetDialog', () => {
  beforeEach(() => {
    Facade.Reset.mockReset()
    Facade.UncommittedCount.mockReset().mockResolvedValue(0)
    confirmRequire.mockReset()
  })

  it('the first line names the branch, the short hash and the subject', async () => {
    const { body } = await mountDialog()

    expect(body.text()).toContain('Reset main to abc123 Fix the bug')
  })

  it('the four modes render one per line, each its own block, mixed preselected', async () => {
    const { body } = await mountDialog()

    const modes = body.findAll('.reset-mode')
    expect(modes.map((m) => m.text().split(' ')[0])).toEqual(['Soft', 'Mixed', 'Hard', 'Keep'])
    expect(body.get<HTMLInputElement>('input#reset-mode-mixed').element.checked).toBe(true)
  })

  it('Reset fails: the dialog stays open with the error, no notice', async () => {
    Facade.Reset.mockRejectedValue(new Error('boom'))
    const { wrapper, body: dialog } = await mountDialog()
    const notice = await mountNotices()

    await submit(dialog)

    expect(dialog.find('.error-banner').exists()).toBe(true)
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it('renders "HEAD" instead of a blank branch name while detached', async () => {
    const { body } = await mountDialog('')

    expect(body.text()).toContain('HEAD')
  })

  it.each(['mixed', 'keep'])(
    'clicking Reset at %s mode calls Facade.Reset directly, no confirmation',
    async (mode) => {
      Facade.Reset.mockResolvedValue({ previousHead: 'def456' })
      const { body } = await mountDialog()

      await body.get(`input#reset-mode-${mode}`).setValue()
      await submit(body)

      expect(Facade.Reset).toHaveBeenCalledTimes(1)
      expect(Facade.Reset).toHaveBeenCalledWith('/repo', 'abc123', mode)
      expect(confirmRequire).not.toHaveBeenCalled()
    },
  )

  it('a successful reset closes the dialog and its notice names the previous short hash, Copy hash copies the full one', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    Facade.Reset.mockResolvedValue({ previousHead: 'deadbeefcafe' })
    const { wrapper, body: dialog } = await mountDialog()
    const notice = await mountNotices()

    await submit(dialog)

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(notice.text()).toContain('deadbee') // the previous commit's short hash

    await notice.get('[data-test="notice-copy"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('deadbeefcafe')
  })

  it("a second reset replaces the notice with its own and leaves another action's notice open", async () => {
    const notice = await mountNotices()
    const OtherAction = defineComponent({
      setup() {
        useToast().add({ group: NOTICE_GROUP, summary: 'Push was rejected', closable: true })
        return () => null
      },
    })
    mount(OtherAction)
    await flushPromises()

    await resetTo('first0000000')
    expect(notice.findAll('.p-toast-message')).toHaveLength(2)
    expect(notice.text()).toContain('first00')

    await resetTo('second0000000')

    expect(notice.findAll('.p-toast-message')).toHaveLength(2)
    expect(notice.text()).toContain('Push was rejected')
    expect(notice.text()).toContain('second0')
    expect(notice.text()).not.toContain('first00')
  })

  it('hard mode confirms first, naming the uncommitted count, before calling Facade.Reset', async () => {
    Facade.UncommittedCount.mockResolvedValue(4)
    Facade.Reset.mockResolvedValue({ previousHead: 'def456' })
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await submit(body)

    expect(Facade.Reset).not.toHaveBeenCalled()
    expect(confirmRequire).toHaveBeenCalledTimes(1)
    expect(confirmOptions().message).toContain('4')

    confirmOptions().accept()
    await flushPromises()

    expect(Facade.Reset).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).toHaveBeenCalledWith('/repo', 'abc123', 'hard')
  })

  it('never calls Facade.Reset if the confirmation is not accepted (QA-SAFE-01)', async () => {
    Facade.UncommittedCount.mockResolvedValue(4)
    const { body } = await mountDialog()

    await body.get('input#reset-mode-hard').setValue()
    await submit(body)

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    expect(Facade.Reset).not.toHaveBeenCalled()
  })
})
