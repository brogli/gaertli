// Requirements: FR-CONF-09, FR-SYNC-01, FR-SYNC-02, FR-SYNC-03, FR-SYNC-06, FR-SYNC-11, FR-UX-01, FR-UX-03, FR-UX-05, FR-UX-09, QA-SAFE-01
import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock(
  '../../../bindings/github.com/brogli/gaertli/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { pressKey } from '@/testing/pressKey'
import { mountNotices } from '@/testing/mountNotices'
import { wailsError } from '@/testing/errors'
import { AppService, Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

// A reactive stand-in for the settings store: settingsStore.settings can be
// reassigned mid-test to prove the button follows a live change, the way a
// save in another window reaches this one (stores/settings.ts). save mimics
// the real store's: it applies the next value, so a menu choice shows.
import type { Preferences } from '../../../bindings/github.com/brogli/gaertli/internal/settings'
const settingsStore = reactive<{ settings: Preferences | null; save: ReturnType<typeof vi.fn> }>({
  settings: null,
  save: vi.fn(),
})

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStore,
}))

import { changedFilesFixture } from '@/testing/fixtures/changes'
import { mountInBody } from '@/testing/mountInBody'
import { clickMenuItem, menuLabels } from '@/testing/contextMenu'
import SyncControls from '../SyncControls.vue'
import { useJobsStore } from '@/stores/jobs'
import { useRefsStore } from '@/stores/refs'
import {
  Code,
  JobKind,
  JobState,
  PullMode,
  type Branch,
} from '../../../bindings/github.com/brogli/gaertli/internal/core'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { findButton } from '@/testing/findButton'
import { refsFixture } from '@/testing/fixtures/refs'
import { jobFixture } from '@/testing/fixtures/job'

function mountControls(props: { compact?: boolean } = {}) {
  return mountInBody(SyncControls, { props: { wt: '/repo', ...props } })
}

// setRefs gives the repo one current branch and the named remotes.
function setRefs(branch: Partial<Branch> = {}, remotes = ['origin']): void {
  useRefsStore().refs = refsFixture(branch, remotes)
}

// rejectPush ends a push job rejected as not fast-forward. Force push shows
// only as the rejection notice's action; useJobNotices renders it.
async function rejectPush(): Promise<void> {
  useJobsStore().upsert(
    jobFixture({
      kind: JobKind.PushJob,
      state: JobState.JobFailed,
      error: { code: Code.NotFastForward, message: 'Updates were rejected' },
    }),
  )
  await flushPromises()
}

// conflictedPull is pull job id stopped on conflicts.
function conflictedPull(id: string) {
  return jobFixture({
    id,
    kind: JobKind.PullJob,
    state: JobState.JobFailed,
    error: { code: Code.Conflicted, message: 'conflicts' },
  })
}

// openPullMenu opens the Pull SplitButton's mode menu.
async function openPullMenu(body: DOMWrapper<Element>): Promise<void> {
  await body.get('.p-splitbutton-dropdown').trigger('click')
  await flushPromises()
}

// choosePullMode picks mode in the Pull SplitButton's menu.
async function choosePullMode(body: DOMWrapper<Element>, mode: string): Promise<void> {
  await openPullMenu(body)
  await clickMenuItem(body, mode, 'tieredmenu')
  await flushPromises()
}

describe('SyncControls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    resetMainWindowApp()
    Facade.Fetch.mockResolvedValue('job-1')
    Facade.Pull.mockResolvedValue('job-1')
    Facade.Push.mockResolvedValue('job-1')
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: false })
    Facade.ForcePush.mockResolvedValue('job-1')
    Facade.PullWithStash.mockResolvedValue('job-1')
    Facade.Status.mockResolvedValue(
      changedFilesFixture({ conflicted: [{ path: 'a.txt', status: 'UU' }] }),
    )
    confirmRequire.mockReset()
    setRefs()
    settingsStore.settings = defaultSettingsFixture()
    settingsStore.save.mockReset().mockImplementation((next: Preferences) => {
      settingsStore.settings = next
    })
  })

  it('clicking Fetch calls Facade.Fetch with the worktree id and no remote', async () => {
    const { body } = await mountControls()

    await findButton(body, 'Fetch')!.trigger('click')

    expect(Facade.Fetch).toHaveBeenCalledTimes(1)
    expect(Facade.Fetch).toHaveBeenCalledWith('/repo', '')
  })

  it('a pull this window started that stops on conflicts opens the diff window; one it did not start opens nothing', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()
    await body.get('.p-splitbutton-button').trigger('click')
    await flushPromises()

    useJobsStore().upsert(conflictedPull('other-window'))
    await flushPromises()
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    useJobsStore().upsert(conflictedPull('job-1'))
    await flushPromises()
    expect(AppService.OpenDiff).toHaveBeenCalledTimes(1)
  })

  it('a pull that stops on conflicts before its start call returns opens the diff window', async () => {
    setRefs({ upstream: 'origin/main' })
    let answerPull: (id: string) => void = () => undefined
    Facade.Pull.mockReturnValueOnce(new Promise<string>((resolve) => (answerPull = resolve)))
    const { body } = await mountControls()
    await flushPromises()
    await body.get('.p-splitbutton-button').trigger('click')

    useJobsStore().upsert(conflictedPull('job-1'))
    await flushPromises()
    answerPull('job-1')
    await flushPromises()

    expect(AppService.OpenDiff).toHaveBeenCalledTimes(1)
  })

  it('a pull that ended is forgotten: a later conflicted end with its id opens nothing', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()
    await body.get('.p-splitbutton-button').trigger('click')
    await flushPromises()

    useJobsStore().upsert(
      jobFixture({ id: 'job-1', kind: JobKind.PullJob, state: JobState.JobDone }),
    )
    await flushPromises()
    useJobsStore().upsert(conflictedPull('job-1'))
    await flushPromises()

    expect(AppService.OpenDiff).not.toHaveBeenCalled()
  })

  it('without a remote, the disabled Pull and Push keep their icon', async () => {
    setRefs({}, [])
    const { body } = await mountControls()

    expect(findButton(body, 'Pull')!.get('.p-button-icon').classes()).toContain('oi-download')
    expect(findButton(body, 'Push')!.get('.p-button-icon').classes()).toContain('oi-upload')
  })

  it.each([false, true])(
    'without a remote, Fetch, Pull and Push are disabled with the no-remote tooltip; no select (compact: %s)',
    async (compact) => {
      setRefs({}, [])
      const { body } = await mountControls({ compact })

      for (const label of ['Fetch', 'Pull', 'Push']) {
        const button = findButton(body, label)!
        expect(button.element.disabled).toBe(true)
        expect(button.attributes('title')).toBe('This repo has no remote')
      }
      expect(body.find('select').exists()).toBe(false)
      expect(body.find('[data-test="upstream-remote"]').exists()).toBe(false)
      expect(findButton(body, 'Pull')!.find('.p-button-label').exists()).toBe(!compact)
    },
  )

  it('detached HEAD (a remote exists): Pull and Push disabled, tooltip names the reason', async () => {
    setRefs({ current: false })
    const { body } = await mountControls()

    expect(findButton(body, 'Fetch')!.element.disabled).toBe(false)
    for (const label of ['Pull', 'Push']) {
      const button = findButton(body, label)!
      expect(button.element.disabled).toBe(true)
      expect(button.attributes('title')).toBe('No branch is checked out')
    }
  })

  it('branch without upstream: the Push button reads "Push"; no remote select in the toolbar', async () => {
    setRefs({ upstream: '' }, ['origin', 'upstream'])
    const { body } = await mountControls()

    const push = findButton(body, 'Push')!
    expect(push.element.disabled).toBe(false)
    expect(push.attributes('aria-label')).toBe('Push')
    expect(body.find('[data-test="upstream-remote"]').exists()).toBe(false)
  })

  it('below wide, the Pull split button shows icon only, its mode in title and aria-label', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls({ compact: true })

    const main = body.get('.p-splitbutton-button')
    expect(main.text()).toBe('')
    expect(main.attributes('title')).toBe('Pull (merge) · Ctrl+T')
    expect(main.attributes('aria-label')).toBe('Pull (merge)')
  })

  it.each([
    ['with an upstream', { upstream: 'origin/main' }, ['origin']],
    ['without an upstream', { upstream: '' }, ['origin', 'upstream']],
  ])(
    'below wide, Push shows icon only with aria-label "Push", %s',
    async (_name, branch, remotes) => {
      setRefs(branch, remotes)
      const { body } = await mountControls({ compact: true })

      const push = findButton(body, 'Push')!
      expect(push.text()).toBe('')
      expect(push.attributes('aria-label')).toBe('Push')
    },
  )

  it('branch without upstream, remote with no remote branches: Push is enabled; PushTarget cannot push straight away, so clicking it opens the dialog naming the branch; confirming pushes to it', async () => {
    setRefs({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: false,
    })
    const { body } = await mountControls()

    const push = findButton(body, 'Push')!
    expect(push.element.disabled).toBe(false)

    await push.trigger('click')
    await flushPromises()

    expect(Facade.Push).not.toHaveBeenCalled()
    expect(body.text()).toContain('origin/main')
    await body.get('[data-test="set-upstream-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', 'main')
  })

  it('Ctrl+Shift+K pushes without an upstream too, when PushTarget can push straight away', async () => {
    setRefs({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: true,
      branch: 'main',
      hasUpstream: false,
    })
    await mountControls()

    pressKey('K', { ctrlKey: true, shiftKey: true })
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', 'main')
  })

  it('force push on a protected branch confirms first, naming it protected', async () => {
    setRefs({ name: 'main', upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ protectedBranches: ['main'] })
    const notice = await mountNotices()
    await mountControls()

    await rejectPush()

    await findButton(notice, 'Force push')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(options.message).toContain('protected')
    expect(Facade.ForcePush).not.toHaveBeenCalled()

    options.accept()
    expect(Facade.ForcePush).toHaveBeenCalledTimes(1)
  })

  it('the button shows and runs Settings’ default pull mode', async () => {
    setRefs({ upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    const { body } = await mountControls()
    await flushPromises()

    expect(findButton(body, 'Pull (rebase)')).toBeTruthy()
    await findButton(body, 'Pull (rebase)')!.trigger('click')

    expect(Facade.Pull).toHaveBeenCalledWith('/repo', PullMode.PullRebase)
  })

  it('choosing "Pull (rebase)" in the menu saves it as the default pull mode; it does not pull', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()

    await choosePullMode(body, 'Pull (rebase)')

    expect(Facade.Pull).not.toHaveBeenCalled()
    expect(settingsStore.save).toHaveBeenCalledTimes(1)
    expect(settingsStore.save).toHaveBeenCalledWith(
      defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase }),
    )
    const pullButton = findButton(body, 'Pull (rebase)')!
    expect(pullButton).toBeTruthy()
    expect(pullButton.attributes('title')).toBe('Pull (rebase) · Ctrl+T')
  })

  it('Ctrl+T pulls with the mode chosen in the menu', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()
    await choosePullMode(body, 'Pull (rebase)')

    pressKey('t', { ctrlKey: true })
    await flushPromises()

    expect(Facade.Pull).toHaveBeenCalledTimes(1)
    expect(Facade.Pull).toHaveBeenCalledWith('/repo', PullMode.PullRebase)
  })

  it('the menu lists "Pull (merge)" and "Pull (rebase)"; only the current mode has an icon; the dropdown is named "Choose pull mode"', async () => {
    setRefs({ upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    const { body } = await mountControls()
    await flushPromises()

    await openPullMenu(body)

    expect(menuLabels(body, 'tieredmenu')).toEqual(['Pull (merge)', 'Pull (rebase)'])
    const items = body.findAll('.p-tieredmenu-item-content')
    expect(items[0]!.find('.p-tieredmenu-item-icon').exists()).toBe(false)
    expect(items[1]!.get('.p-tieredmenu-item-icon').classes()).toContain('oi-check')
    const dropdown = body.get('.p-splitbutton-dropdown')
    expect(dropdown.attributes('title')).toBe('Choose pull mode')
    expect(dropdown.attributes('aria-label')).toBe('Choose pull mode')
  })

  it('a failed save shows in the error banner; Facade.Pull is not called and the button keeps its mode', async () => {
    setRefs({ upstream: 'origin/main' })
    settingsStore.save.mockRejectedValueOnce(wailsError('disk full'))
    const { body } = await mountControls()
    await flushPromises()

    await choosePullMode(body, 'Pull (rebase)')

    expect(body.get('.error-banner').text()).toContain('disk full')
    expect(Facade.Pull).not.toHaveBeenCalled()
    expect(findButton(body, 'Pull (merge)')).toBeTruthy()
  })

  it('a Settings default saved in another window updates the button without a remount', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()
    expect(findButton(body, 'Pull (merge)')).toBeTruthy()

    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    await flushPromises()

    expect(findButton(body, 'Pull (rebase)')).toBeTruthy()
  })

  it('a rejected Fetch renders ErrorBanner with the error', async () => {
    Facade.Fetch.mockRejectedValueOnce(wailsError('network unreachable'))
    const { body } = await mountControls()

    await findButton(body, 'Fetch')!.trigger('click')
    await flushPromises()

    expect(body.get('.error-banner').text()).toContain('network unreachable')
  })

  it('Ctrl+Shift+K pushes, same as clicking Push', async () => {
    setRefs({ upstream: 'origin/main' })
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: true,
    })
    await mountControls()

    pressKey('K', { ctrlKey: true, shiftKey: true })
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', '', '')
  })

  it('Pull and Push tooltips name their shortcut keys; aria-labels stay the actions alone', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()

    expect(findButton(body, 'Pull (merge)')!.attributes('title')).toBe('Pull (merge) · Ctrl+T')
    expect(findButton(body, 'Pull (merge)')!.attributes('aria-label')).toBe('Pull (merge)')
    expect(findButton(body, 'Push')!.attributes('title')).toBe('Push · Ctrl+Shift+K')
    expect(findButton(body, 'Push')!.attributes('aria-label')).toBe('Push')
  })

  it.each([
    ['without an upstream', { upstream: '' }, ['origin']],
    ['without a remote, even with an upstream', { upstream: 'origin/main' }, []],
  ])('Ctrl+T does nothing %s (Pull is not available)', async (_name, branch, remotes) => {
    setRefs(branch, remotes)
    await mountControls()

    pressKey('t', { ctrlKey: true })
    await flushPromises()

    expect(Facade.Pull).not.toHaveBeenCalled()
  })
})
