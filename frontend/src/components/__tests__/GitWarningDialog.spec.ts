// Requirements: QA-COMP-04
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { AppService } = vi.hoisted(() => ({ AppService: { TakeGitWarnings: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ AppService }))
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import GitWarningDialog from '../GitWarningDialog.vue'
import { useRepoStore } from '@/stores/repo'
import { summaryFixture } from '@/testing/fixtures/summary'
import { mountInBody } from '@/testing/mountInBody'
import { footerButtons } from '@/testing/dialog'

const hostWarning = { version: '2.43.0', latest: '2.56', distro: '' }
const distroWarning = { version: '', latest: '2.56', distro: 'Ubuntu' }

describe('GitWarningDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.TakeGitWarnings.mockReset().mockResolvedValue(null)
  })

  it('warns about an outdated git, naming both versions and where to update', async () => {
    AppService.TakeGitWarnings.mockResolvedValue([hostWarning])

    const { body } = await mountInBody(GitWarningDialog)

    const dialog = body.get('.p-dialog')
    expect(dialog.text()).toContain('git is out of date')
    expect(dialog.text()).toContain(
      'Gärtli supports git 2.56 and newer. The installed git is 2.43.0; some features may not work.',
    )
    expect(dialog.get('a').attributes('href')).toBe('https://git-scm.com/downloads')
    expect(footerButtons(body)).toEqual(['OK'])
  })

  it('names the WSL distro whose git is outdated, and an unreadable version as unknown', async () => {
    AppService.TakeGitWarnings.mockResolvedValue([distroWarning])

    const { body } = await mountInBody(GitWarningDialog)

    expect(body.get('.p-dialog').text()).toContain(
      'The git in WSL distro Ubuntu is of an unknown version; some features may not work.',
    )
  })

  it('shows every outdated git in one warning', async () => {
    AppService.TakeGitWarnings.mockResolvedValue([hostWarning, distroWarning])

    const { body } = await mountInBody(GitWarningDialog)

    expect(body.findAll('.p-dialog')).toHaveLength(1)
    expect(body.get('.p-dialog').text()).toContain('The installed git is 2.43.0')
    expect(body.get('.p-dialog').text()).toContain('The git in WSL distro Ubuntu')
  })

  it("asks again once the window's repo has loaded, whose first use may find an outdated git", async () => {
    const { body } = await mountInBody(GitWarningDialog)
    expect(body.find('.p-dialog').exists()).toBe(false)

    AppService.TakeGitWarnings.mockResolvedValue([distroWarning])
    useRepoStore().summary = summaryFixture()
    await flushPromises()

    expect(body.get('.p-dialog').text()).toContain('The git in WSL distro Ubuntu')
  })

  it('OK closes the warning', async () => {
    AppService.TakeGitWarnings.mockResolvedValue([hostWarning])
    const { body } = await mountInBody(GitWarningDialog)

    await body.get('[data-test="git-warning-ok"]').trigger('click')
    await flushPromises()

    expect(body.find('.p-dialog').exists()).toBe(false)
  })

  it('shows nothing for a current git', async () => {
    const { body } = await mountInBody(GitWarningDialog)

    expect(AppService.TakeGitWarnings).toHaveBeenCalledTimes(1)
    expect(body.find('.p-dialog').exists()).toBe(false)
  })
})
