import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, SettingsService } = vi.hoisted(() => ({
  Facade: { Squash: vi.fn(), Fixup: vi.fn() },
  SettingsService: { Get: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade,
  SettingsService,
}))

import SquashDialog from '../SquashDialog.vue'
import NoticeArea from '../NoticeArea.vue'
import { mountInBody } from '@/testing/mountInBody'
import { field, fieldLabels, footerButtons } from '@/testing/dialog'
import { useRepoStore } from '@/stores/repo'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { summaryFixture } from '@/testing/fixtures/summary'
import { Code, type SquashCheck } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const shas = ['sha-a', 'sha-b', 'sha-c']

function mountDialog(check: Partial<SquashCheck> = {}, fixup = false) {
  return mountInBody(SquashDialog, {
    props: { wt: '/repo', shas, check: { pushed: false, message: 'A\n\nB', ...check }, fixup },
  })
}

function onBranch(branch: string, protectedBranches: string[] = []): void {
  useRepoStore().summary = summaryFixture({ branch })
  useSettingsStore().settings = defaultSettingsFixture({ protectedBranches })
}

describe('SquashDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    onBranch('feature')
    Facade.Squash.mockReset().mockResolvedValue({ previousHead: 'deadbeef1234' })
    Facade.Fixup.mockReset().mockResolvedValue({ previousHead: 'deadbeef1234' })
  })

  it('FR-LOG-14 squash shows the combined message for editing and squashes with the edited one', async () => {
    const { wrapper, body } = await mountDialog()
    const field = body.get<HTMLTextAreaElement>('[data-test="squash-message"]')
    expect(field.element.value).toBe('A\n\nB')

    await field.setValue('Edited')
    await body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Squash).toHaveBeenCalledWith('/repo', shas, 'Edited')
    expect(wrapper.emitted('done')).toHaveLength(1)
  })

  it('FR-UX-17 the message has its own label; Cancel and Squash sit together at the bottom; Cancel closes', async () => {
    const { wrapper, body } = await mountDialog()

    expect(fieldLabels(body)).toEqual(['Message of the new commit'])
    expect(field(body, 'Message of the new commit').element.value).toBe('A\n\nB')
    expect(footerButtons(body)).toEqual(['Cancel', 'Squash'])
    await body.get('[data-test="dialog-cancel"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-LOG-14 a blank message disables Squash', async () => {
    const { body } = await mountDialog()

    await body.get('[data-test="squash-message"]').setValue('  \n')

    expect(body.get('[data-test="squash-submit"]').attributes('disabled')).toBeDefined()
  })

  it('FR-LOG-14 fixup asks first, with no message field, and keeps the oldest message', async () => {
    const { body } = await mountDialog({}, true)

    expect(body.find('[data-test="squash-message"]').exists()).toBe(false)
    expect(body.text()).toContain('Fixup 3 commits')
    expect(body.text()).toContain('with the message of the oldest')
    expect(Facade.Fixup).not.toHaveBeenCalled()

    await body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Fixup).toHaveBeenCalledWith('/repo', shas)
  })

  it('FR-LOG-14 a pushed commit is named, with the protected-branch warning on a protected branch', async () => {
    const unpushed = await mountDialog()
    expect(unpushed.body.text()).not.toContain('already pushed')
    unpushed.wrapper.unmount()

    onBranch('main', ['main'])
    const { body } = await mountDialog({ pushed: true })

    expect(body.text()).toContain('already pushed: afterwards the branch needs a force push')
    expect(body.text()).toContain('"main" is a protected branch.')
  })

  it('FR-LOG-14 success leaves a notice naming the branch and the previous commit', async () => {
    const { body: notice } = await mountInBody(NoticeArea)
    const { body } = await mountDialog()

    await body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('Squashed 3 commits on feature. Previous commit: deadbee')
  })

  it("FR-LOG-14 a stop that is no conflict closes the dialog and names git's reason in a notice", async () => {
    const stop = {
      code: Code.Stopped,
      message: 'untracked working tree files would be overwritten by merge',
    }
    Facade.Squash.mockRejectedValueOnce(new Error('x', { cause: stop }))
    const { body: notice } = await mountInBody(NoticeArea)
    const { wrapper, body } = await mountDialog()

    await body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('done')).toHaveLength(1)
    expect(notice.text()).toContain('untracked working tree files would be overwritten by merge')
  })

  it('FR-LOG-14 a conflict stop closes the dialog for the conflict banner; another error stays in it', async () => {
    const conflict = { code: Code.Conflicts, message: 'could not apply' }
    Facade.Squash.mockRejectedValueOnce(new Error('x', { cause: conflict }))
    const first = await mountDialog()
    await first.body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()
    expect(first.wrapper.emitted('done')).toHaveLength(1)
    first.wrapper.unmount()

    const refused = { code: Code.GitFailed, message: 'hook said no' }
    Facade.Squash.mockRejectedValueOnce(new Error('x', { cause: refused }))
    const second = await mountDialog()
    await second.body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()
    expect(second.wrapper.emitted('done')).toBeUndefined()
    expect(second.body.text()).toContain('hook said no')
  })
})
