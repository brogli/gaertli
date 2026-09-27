import { flushPromises, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    Fetch: vi.fn(),
    Pull: vi.fn(),
    Push: vi.fn(),
    PushTarget: vi.fn(),
    ForcePush: vi.fn(),
    PullWithStash: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
import { confirmOptions, confirmRequire } from '@/testing/confirm'

// A reactive stand-in for the settings store: settingsStore.settings can be
// reassigned mid-test to prove the button follows a live change, the way a
// save in another window reaches this one (stores/settings.ts).
import type { Preferences } from '../../../bindings/github.com/brogli/nebegit/internal/settings'
const settingsStore = reactive<{ settings: Preferences | null }>({ settings: null })

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStore,
}))

import { createI18n } from 'vue-i18n'
import en from '@/locales/en.json'
import { mountInBody } from '@/testing/mountInBody'
import { clickMenuItem } from '@/testing/contextMenu'
import SyncControls from '../SyncControls.vue'
import NoticeArea from '../NoticeArea.vue'
import { useJobsStore } from '@/stores/jobs'
import { useRefsStore } from '@/stores/refs'
import {
  Code,
  JobKind,
  JobState,
  PullMode,
  type Branch,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { findButton } from '@/testing/findButton'
import { branchFixture } from '@/testing/fixtures/branch'
import { jobFixture } from '@/testing/fixtures/job'

function mountControls(props: { compact?: boolean } = {}) {
  return mountInBody(SyncControls, { props: { wt: '/repo', ...props } })
}

// setRefs gives the repo one current branch and the named remotes.
function setRefs(branch: Partial<Branch> = {}, remotes = ['origin']): void {
  useRefsStore().refs = {
    branches: [branchFixture(branch)],
    remotes: remotes.map((name) => ({ name, branches: [] })),
    tags: [],
  }
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

// openPullMenu opens the Pull SplitButton's mode menu.
async function openPullMenu(body: DOMWrapper<Element>): Promise<void> {
  await body.get('.p-splitbutton-dropdown').trigger('click')
}

describe('SyncControls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    Facade.Fetch.mockReset().mockResolvedValue('job-1')
    Facade.Pull.mockReset().mockResolvedValue('job-1')
    Facade.Push.mockReset().mockResolvedValue('job-1')
    Facade.PushTarget.mockReset().mockResolvedValue({ remote: 'origin', autoSetUpstream: false })
    Facade.ForcePush.mockReset().mockResolvedValue('job-1')
    Facade.PullWithStash.mockReset().mockResolvedValue('job-1')
    confirmRequire.mockReset()
    setRefs()
    settingsStore.settings = defaultSettingsFixture()
  })

  it('FR-UX-11 the Fetch button reads toolbar.fetch from the active locale, not a literal', async () => {
    const xx = createI18n({
      legacy: false,
      locale: 'xx',
      fallbackLocale: 'en',
      messages: { xx: { ...en, toolbar: { ...en.toolbar, fetch: 'XFetch' } } },
    })
    const { body } = await mountInBody(SyncControls, {
      props: { wt: '/repo' },
      global: { plugins: [xx] },
    })

    expect(findButton(body, 'XFetch')).toBeTruthy()
  })

  it('FR-SYNC-01 clicking Fetch calls Facade.Fetch with the worktree id and no remote', async () => {
    const { body } = await mountControls()

    await findButton(body, 'Fetch')!.trigger('click')

    expect(Facade.Fetch).toHaveBeenCalledTimes(1)
    expect(Facade.Fetch).toHaveBeenCalledWith('/repo', '')
  })

  it('FR-SYNC-11 without a remote, Fetch, Pull and Push are disabled with the no-remote tooltip; no select', async () => {
    setRefs({}, [])
    const { body } = await mountControls()

    for (const label of ['Fetch', 'Pull', 'Push']) {
      const button = findButton(body, label)!
      expect(button.element.disabled).toBe(true)
      expect(button.attributes('title')).toBe('This repo has no remote')
    }
    expect(body.find('select').exists()).toBe(false)
  })

  it('FR-UX-09 without a remote, the disabled Pull and Push are OpenVue Buttons with their icon, secondary and outlined', async () => {
    setRefs({}, [])
    const { body } = await mountControls()

    const pull = findButton(body, 'Pull')!
    const push = findButton(body, 'Push')!
    expect(pull.get('.p-button-icon').classes()).toContain('oi-download')
    expect(push.get('.p-button-icon').classes()).toContain('oi-upload')
    for (const button of [pull, push]) {
      expect(button.classes()).toContain('p-button-secondary')
      expect(button.classes()).toContain('p-button-outlined')
    }
    expect(body.find('[data-test="upstream-remote"]').exists()).toBe(false)
  })

  it('FR-SYNC-11 detached HEAD (a remote exists): Pull and Push disabled, tooltip names the reason', async () => {
    setRefs({ current: false })
    const { body } = await mountControls()

    expect(findButton(body, 'Fetch')!.element.disabled).toBe(false)
    for (const label of ['Pull', 'Push']) {
      const button = findButton(body, label)!
      expect(button.element.disabled).toBe(true)
      expect(button.attributes('title')).toBe('No branch is checked out')
    }
  })

  it('FR-UX-09 branch without upstream: the Push button reads "Push", not the remote name; no remote select in the toolbar', async () => {
    setRefs({ upstream: '' }, ['origin', 'upstream'])
    const { body } = await mountControls()

    const push = findButton(body, 'Push')!
    expect(push.element.disabled).toBe(false)
    expect(push.attributes('title')).toBe('Push')
    expect(push.attributes('aria-label')).toBe('Push')
    expect(body.find('[data-test="upstream-remote"]').exists()).toBe(false)
  })

  it('FR-UX-09 below wide, the Pull split button shows icon only, its mode in title and aria-label', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls({ compact: true })

    const main = body.get('.p-splitbutton-button')
    expect(main.text()).toBe('')
    expect(main.attributes('title')).toBe('Pull (merge)')
    expect(main.attributes('aria-label')).toBe('Pull (merge)')
  })

  it('FR-UX-09 the Pull split button matches its plain neighbours: secondary and outlined, not OpenVue’s primary colour', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()

    for (const part of ['.p-splitbutton-button', '.p-splitbutton-dropdown']) {
      const classes = body.get(part).classes()
      expect(classes).toContain('p-button-secondary')
      expect(classes).toContain('p-button-outlined')
    }
  })

  it('FR-UX-09 Fetch and Push are OpenVue Buttons, secondary and outlined', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()

    for (const label of ['Fetch', 'Push']) {
      const classes = findButton(body, label)!.classes()
      expect(classes).toContain('p-button')
      expect(classes).toContain('p-button-secondary')
      expect(classes).toContain('p-button-outlined')
    }
  })

  it('FR-UX-09 below wide, Push shows icon only with title and aria-label "Push", with or without an upstream', async () => {
    setRefs({ upstream: 'origin/main' })
    const withUpstream = await mountControls({ compact: true })
    const push = findButton(withUpstream.body, 'Push')!
    expect(push.attributes('title')).toBe('Push')
    expect(push.attributes('aria-label')).toBe('Push')
    expect(push.find('.p-button-label').exists()).toBe(false)

    setRefs({ upstream: '' }, ['origin', 'upstream'])
    const noUpstream = await mountControls({ compact: true })
    const pushNoUpstream = findButton(noUpstream.body, 'Push')!
    expect(pushNoUpstream.attributes('title')).toBe('Push')
    expect(pushNoUpstream.attributes('aria-label')).toBe('Push')
    expect(pushNoUpstream.find('.p-button-label').exists()).toBe(false)
  })

  it('FR-UX-09 below wide, the disabled Pull and Push also show icon only, title unchanged', async () => {
    setRefs({}, [])
    const { body } = await mountControls({ compact: true })

    for (const label of ['Pull', 'Push']) {
      const button = findButton(body, label)!
      expect(button.attributes('title')).toBe('This repo has no remote')
      expect(button.find('.p-button-label').exists()).toBe(false)
    }
  })

  it('FR-SYNC-03 branch without upstream, PushTarget cannot push straight away: clicking Push opens the set-upstream dialog; confirming pushes to the target remote', async () => {
    setRefs({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: false })
    const { body } = await mountControls()

    await findButton(body, 'Push')!.trigger('click')
    await flushPromises()

    expect(Facade.Push).not.toHaveBeenCalled()
    await body.get('[data-test="set-upstream-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', '')
  })

  it('FR-SYNC-03 Ctrl+Shift+K pushes without an upstream too, when PushTarget can push straight away (old: nothing)', async () => {
    setRefs({ upstream: '' })
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: true })
    await mountControls()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true, shiftKey: true }))
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', '')
  })

  it('FR-SYNC-06, FR-UX-03, QA-SAFE-01 force push on a protected branch confirms first, naming it protected', async () => {
    setRefs({ name: 'main', upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ protectedBranches: ['main'] })
    const { body: notice } = await mountInBody(NoticeArea)
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

  it('FR-SYNC-06 cancelling the force-push confirmation calls nothing; the notice still offers it', async () => {
    setRefs({ name: 'main', upstream: 'origin/main' })
    const { body: notice } = await mountInBody(NoticeArea)
    await mountControls()

    await rejectPush()

    await findButton(notice, 'Force push')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    expect(Facade.ForcePush).not.toHaveBeenCalled()

    // Cancelling means never accepting; the click itself never closed the
    // notice, so it still offers Force push and nothing ran.
    expect(findButton(notice, 'Force push')).toBeTruthy()
    expect(Facade.ForcePush).not.toHaveBeenCalled()
  })

  it('FR-SYNC-02 the button shows and runs Settings’ default pull mode', async () => {
    setRefs({ upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    const { body } = await mountControls()
    await flushPromises()

    expect(findButton(body, 'Pull (rebase)')).toBeTruthy()
    await findButton(body, 'Pull (rebase)')!.trigger('click')

    expect(Facade.Pull).toHaveBeenCalledWith('/repo', PullMode.PullRebase)
  })

  it('FR-SYNC-02 choosing "Pull with merge" from the SplitButton menu runs it once without changing the button', async () => {
    setRefs({ upstream: 'origin/main' })
    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    const { body } = await mountControls()
    await flushPromises()

    await openPullMenu(body)
    await flushPromises()
    await clickMenuItem(body, 'Pull with merge', 'tieredmenu')
    await flushPromises()

    expect(Facade.Pull).toHaveBeenCalledTimes(1)
    expect(Facade.Pull).toHaveBeenCalledWith('/repo', PullMode.PullMerge)
    expect(findButton(body, 'Pull (rebase)')).toBeTruthy()
  })

  it('FR-SYNC-02 a menu choice is not remembered: a reload shows Settings’ default again', async () => {
    setRefs({ upstream: 'origin/main' })
    const first = await mountControls()
    await flushPromises()
    await openPullMenu(first.body)
    await clickMenuItem(first.body, 'Pull with rebase', 'tieredmenu')
    first.wrapper.unmount()

    const second = await mountControls()
    await flushPromises()

    expect(findButton(second.body, 'Pull (merge)')).toBeTruthy()
  })

  it('FR-SYNC-02 a Settings default saved in another window updates the button without a remount', async () => {
    setRefs({ upstream: 'origin/main' })
    const { body } = await mountControls()
    await flushPromises()
    expect(findButton(body, 'Pull (merge)')).toBeTruthy()

    settingsStore.settings = defaultSettingsFixture({ defaultPullMode: PullMode.PullRebase })
    await flushPromises()

    expect(findButton(body, 'Pull (rebase)')).toBeTruthy()
  })

  it('FR-UX-01 a rejected Fetch renders ErrorBanner with the error', async () => {
    Facade.Fetch.mockRejectedValueOnce(
      new Error('boom', { cause: { message: 'network unreachable' } }),
    )
    const { body } = await mountControls()

    await findButton(body, 'Fetch')!.trigger('click')
    await flushPromises()

    expect(body.get('.error-banner').text()).toContain('network unreachable')
  })

  it('FR-UX-05 Ctrl+T pulls, same as clicking Pull', async () => {
    setRefs({ upstream: 'origin/main' })
    await mountControls()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 't', ctrlKey: true }))
    await flushPromises()

    expect(Facade.Pull).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-05 Ctrl+Shift+K pushes, same as clicking Push', async () => {
    setRefs({ upstream: 'origin/main' })
    await mountControls()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true, shiftKey: true }))
    await flushPromises()

    expect(Facade.Push).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-05 Ctrl+T does nothing without an upstream (no Pull to gate)', async () => {
    setRefs({ upstream: '' })
    await mountControls()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 't', ctrlKey: true }))
    await flushPromises()

    expect(Facade.Pull).not.toHaveBeenCalled()
  })
})
