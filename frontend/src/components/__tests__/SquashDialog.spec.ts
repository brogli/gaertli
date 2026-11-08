// Requirements: FR-LOG-14, FR-LOG-18, FR-LOG-19, FR-LOG-20
import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, SettingsService, AppService } = vi.hoisted(() => ({
  Facade: { Squash: vi.fn(), Fixup: vi.fn(), Status: vi.fn() },
  AppService: { OpenDiff: vi.fn() },
  SettingsService: { Get: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({
  Facade,
  SettingsService,
  AppService,
}))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import SquashDialog from '../SquashDialog.vue'
import { mountNotices } from '@/testing/mountNotices'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { mountInBody } from '@/testing/mountInBody'
import { wailsError } from '@/testing/errors'
import { useRepoStore } from '@/stores/repo'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { summaryFixture } from '@/testing/fixtures/summary'
import { Code, type SquashCheck } from '../../../bindings/github.com/brogli/gaertli/internal/core'

const shas = ['sha-a', 'sha-b', 'sha-c']

function mountDialog(check: Partial<SquashCheck> = {}, fixup = false) {
  return mountInBody(SquashDialog, {
    props: { wt: '/repo', shas, check: { pushed: false, message: 'A\n\nB', ...check }, fixup },
  })
}

async function submit(body: DOMWrapper<Element>): Promise<void> {
  await body.get('[data-test="squash-submit"]').trigger('click')
  await flushPromises()
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
    Facade.Status.mockReset().mockResolvedValue(
      changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] }),
    )
    AppService.OpenDiff.mockReset().mockResolvedValue(undefined)
  })

  it('squash shows the combined message for editing and squashes with the edited one; a blank message disables Squash', async () => {
    const { wrapper, body } = await mountDialog()
    const field = body.get<HTMLTextAreaElement>('[data-test="squash-message"]')
    expect(field.element.value).toBe('A\n\nB')

    await field.setValue('  \n')
    expect(body.get('[data-test="squash-submit"]').attributes('disabled')).toBeDefined()

    await field.setValue('Edited')
    await submit(body)

    expect(Facade.Squash).toHaveBeenCalledWith('/repo', shas, 'Edited')
    expect(wrapper.emitted('done')).toHaveLength(1)
  })

  it('fixup asks first, with no message field, and keeps the oldest message', async () => {
    const { body } = await mountDialog({}, true)

    expect(body.find('[data-test="squash-message"]').exists()).toBe(false)
    expect(body.text()).toContain('Fixup 3 commits')
    expect(body.text()).toContain('with the message of the oldest')
    expect(Facade.Fixup).not.toHaveBeenCalled()

    await submit(body)

    expect(Facade.Fixup).toHaveBeenCalledWith('/repo', shas)
  })

  it('a pushed commit is named, with the protected-branch warning on a protected branch', async () => {
    const unpushed = await mountDialog()
    expect(unpushed.body.text()).not.toContain('already pushed')
    unpushed.wrapper.unmount()

    onBranch('main', ['main'])
    const { body } = await mountDialog({ pushed: true })

    expect(body.text()).toContain('already pushed: afterwards the branch needs a force push')
    expect(body.text()).toContain('"main" is a protected branch.')
  })

  it('success leaves a notice naming the branch and the previous commit', async () => {
    const notice = await mountNotices()
    const { body } = await mountDialog()

    await submit(body)

    expect(notice.text()).toContain('Squashed 3 commits on feature. Previous commit: deadbee')
  })

  it.each([
    {
      outcome: "a stop that is no conflict closes the dialog and names git's reason in a notice",
      error: { code: Code.Stopped, message: 'untracked working tree files would be overwritten' },
      closes: true,
      shows: 'untracked working tree files would be overwritten',
    },
    {
      outcome: 'a conflict stop closes the dialog and opens the conflicted file',
      error: { code: Code.Conflicts, message: 'could not apply' },
      closes: true,
      shows: '',
    },
    {
      outcome: 'another error stays in the dialog',
      error: { code: Code.GitFailed, message: 'hook said no' },
      closes: false,
      shows: 'hook said no',
    },
  ] as const)('$outcome', async ({ error, closes, shows }) => {
    Facade.Squash.mockRejectedValueOnce(wailsError(error.message, error.code))
    const notice = await mountNotices()
    const { wrapper, body } = await mountDialog()

    await submit(body)

    expect(wrapper.emitted('done')?.length).toBe(closes ? 1 : undefined)
    expect(notice.text() + body.text()).toContain(shows)
    expect(AppService.OpenDiff).toHaveBeenCalledTimes(error.code === Code.Conflicts ? 1 : 0)
  })
})
