import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { pressKey } from '@/testing/pressKey'
import { Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import CommitBox from '../CommitBox.vue'
import { useChangesStore } from '@/stores/changes'
import { useRepoStore } from '@/stores/repo'
import { useRefsStore } from '@/stores/refs'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { summaryFixture } from '@/testing/fixtures/summary'
import { Operation } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { refsFixture } from '@/testing/fixtures/refs'
import { wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'

const props = { wt: '/repo' }

// amendCheckbox is the Amend checkbox's <input>, the first of the box's
// two checkboxes.
function amendCheckbox(wrapper: VueWrapper) {
  return wrapper.get<HTMLInputElement>('input[type=checkbox]')
}

function submitButton(wrapper: VueWrapper) {
  return wrapper.get<HTMLButtonElement>('[data-test="commit-submit"]')
}

describe('CommitBox', () => {
  // Fake timers throughout: a real 300ms draft-save debounce started in one
  // test would otherwise fire during a later one.
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    resetMainWindowApp()
    Facade.Push.mockResolvedValue('job-1')
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it.each([
    ['disabled with nothing staged, amend unchecked', false, false, 'a message', true],
    ['disabled with a staged file but a blank message', true, false, '', true],
    ['disabled with a staged file but a whitespace-only message', true, false, '   ', true],
    ['enabled with a staged file and a message', true, false, 'm', false],
    ['enabled with nothing staged when amend is checked', false, true, 'a message', false],
  ])(
    'FR-CHG-05 FR-CHG-06 the Commit button is %s',
    async (_name, staged, amend, message, disabled) => {
      useChangesStore().files = changedFilesFixture(
        staged ? { staged: [{ path: 'a.txt', status: 'M' }] } : {},
      )
      useRepoStore().summary = summaryFixture({ sha: 'abc123' })

      const wrapper = mount(CommitBox, { props })
      if (amend) await amendCheckbox(wrapper).setValue(true)
      await wrapper.get('textarea').setValue(message)

      expect(submitButton(wrapper).element.disabled).toBe(disabled)
    },
  )

  it("FR-CHG-07 a rejected commit shows the hook's output and keeps the message", async () => {
    useChangesStore().files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    useRepoStore().summary = summaryFixture({ sha: 'abc123' })
    Facade.Commit.mockRejectedValue(wailsError('hook says no', 'STOPPED'))

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('blocked by the hook')
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(wrapper.get('.error-banner').text()).toContain('hook says no')
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe('blocked by the hook')
  })

  it('FR-CHG-05 clicking Commit calls Facade.Commit with the message, amend false', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await submitButton(wrapper).trigger('click')

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.Commit).toHaveBeenCalledWith('/repo', 'a message', false)
  })

  it('FR-CHG-05 a successful commit clears the message and resets amend', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await amendCheckbox(wrapper).setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(wrapper.get('textarea').element.value).toBe('')
    expect(amendCheckbox(wrapper).element.checked).toBe(false)
  })

  it('FR-CHG-05 the Commit button is disabled while a commit is in flight, so a double-click cannot amend twice', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    let resolveCommit: () => void = () => {}
    Facade.Commit.mockReturnValue(new Promise<void>((resolve) => (resolveCommit = resolve)))

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await submitButton(wrapper).trigger('click')

    expect(submitButton(wrapper).element.disabled).toBe(true)

    resolveCommit()
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['on an unborn HEAD', summaryFixture({ sha: '' })],
    [
      'while an operation (merge, rebase, …) is in progress',
      summaryFixture({ sha: 'abc123', operation: Operation.MergeOperation }),
    ],
  ])('FR-CHG-06 the Amend checkbox is disabled %s', (_name, summary) => {
    useRepoStore().summary = summary

    const wrapper = mount(CommitBox, { props })

    expect(amendCheckbox(wrapper).element.disabled).toBe(true)
  })

  it('FR-CHG-06 checking Amend with an empty message pre-fills it from CommitDetails', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    Facade.CommitDetails.mockResolvedValue({ body: 'last msg' })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)
    await flushPromises()

    expect(Facade.CommitDetails).toHaveBeenCalledTimes(1)
    expect(Facade.CommitDetails).toHaveBeenCalledWith('/repo', 'abc123')
    expect(wrapper.get('textarea').element.value).toBe('last msg')
  })

  it('FR-CHG-06 checking Amend never clobbers a message already typed', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('typed draft')
    await amendCheckbox(wrapper).setValue(true)
    await flushPromises()

    expect(Facade.CommitDetails).not.toHaveBeenCalled()
    expect(wrapper.get('textarea').element.value).toBe('typed draft')
  })

  it.each([
    [
      'warns that an amended, already-pushed commit will need a force push',
      { upstream: 'origin/main', ahead: 0 },
      true,
    ],
    [
      'does not warn when the current branch is ahead of its upstream',
      { upstream: 'origin/main', ahead: 1 },
      false,
    ],
    ['does not warn when the current branch has no upstream', { upstream: '' }, false],
  ])('FR-CHG-06 %s', async (_name, branch, warns) => {
    useRepoStore().summary = summaryFixture({ sha: 'abc123' })
    useRefsStore().refs = refsFixture(branch)

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(wrapper.text().includes('already been pushed')).toBe(warns)
  })

  it('FR-CHG-08 initializes the message field from the restored draft', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ messageDraft: 'restored' })

    const wrapper = mount(CommitBox, { props })

    expect(wrapper.get('textarea').element.value).toBe('restored')
  })

  it('FR-CHG-08 saves the message as a draft, debounced 300ms', async () => {
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('typing...')
    await vi.advanceTimersByTimeAsync(300)

    expect(Facade.SetMessageDraft).toHaveBeenCalledTimes(1)
    expect(Facade.SetMessageDraft).toHaveBeenCalledWith('/repo', 'typing...')
  })

  it('FR-CHG-08 does not re-save a restored draft that was never edited', async () => {
    const repo = useRepoStore()
    const wrapper = mount(CommitBox, { props })
    // Summary (and its draft) arrives asynchronously, after CommitBox has
    // already mounted and is watching `message` for edits to save.
    repo.summary = summaryFixture({ messageDraft: 'restored' })
    await wrapper.vm.$nextTick()
    await vi.advanceTimersByTimeAsync(300)

    expect(Facade.SetMessageDraft).not.toHaveBeenCalled()
  })

  it.each([
    ['commits, same as clicking Commit', true, 1],
    ['with nothing staged does not commit, same gating as the button', false, 0],
  ])('FR-UX-05 Ctrl+K %s', async (_name, staged, commits) => {
    useChangesStore().files = changedFilesFixture(
      staged ? { staged: [{ path: 'a.txt', status: 'M' }] } : {},
    )
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    pressKey('k', { ctrlKey: true })
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(commits)
  })

  it('FR-UX-05 FR-CHG-05 the Commit button shows a Ctrl+K hint at its right; its aria-label stays the label alone', () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({
      staged: [
        { path: 'a.txt', status: 'M' },
        { path: 'b.txt', status: 'M' },
      ],
    })

    const wrapper = mount(CommitBox, { props })
    const button = submitButton(wrapper)

    expect(button.text()).toContain('Commit (2)')
    expect(button.get('[data-test="shortcut-hint"]').text()).toBe('Ctrl+K')
    expect(button.attributes('aria-label')).toBe('Commit (2)')
  })

  it('FR-CHG-09 with an upstream, checking "Push after commit" then clicking Commit calls Facade.Commit then Facade.Push', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' }, ['origin'])
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: true,
    })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.Push).toHaveBeenCalledTimes(1)
    expect(Facade.Push).toHaveBeenCalledWith('/repo', '', '')
    expect(Facade.Commit.mock.invocationCallOrder[0]!).toBeLessThan(
      Facade.Push.mock.invocationCallOrder[0]!,
    )
  })

  it('FR-CHG-09 the "Push after commit" checkbox is enabled without an upstream when a remote exists, disabled without one', async () => {
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' }, ['origin'])

    const wrapper = mount(CommitBox, { props })

    expect(
      wrapper.get<HTMLInputElement>('[data-test="push-after-commit"] input').element.disabled,
    ).toBe(false)

    refs.refs = refsFixture({ upstream: '' }, [])
    await wrapper.vm.$nextTick()

    expect(
      wrapper.get<HTMLInputElement>('[data-test="push-after-commit"] input').element.disabled,
    ).toBe(true)
  })

  it('FR-CHG-09 without an upstream, checking "Push after commit" and committing pushes the way PushTarget says', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' }, ['origin'])
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: true,
      branch: 'main',
      hasUpstream: false,
    })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.PushTarget).toHaveBeenCalledWith('/repo')
    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin', 'main')
  })

  it('FR-CHG-09 without an upstream, committing while PushTarget cannot decide opens the dialog; Cancel leaves the commit made, unpushed', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' }, ['origin'])
    Facade.PushTarget.mockResolvedValue({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: false,
    })

    const { wrapper, body } = await mountInBody(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(body.find('[data-test="set-upstream-confirm"]').exists()).toBe(true)

    await body.get('[data-test="dialog-cancel"]').trigger('click')

    expect(Facade.Push).not.toHaveBeenCalled()
  })

  it('FR-CHG-09 Push after commit is disabled once Amend is checked; checking Push after commit, then Amend, then committing never pushes (amend needs a force push)', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' }, ['origin'])

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await amendCheckbox(wrapper).setValue(true) // Amend, ticked after Push after commit
    expect(
      wrapper.get<HTMLInputElement>('[data-test="push-after-commit"] input').element.disabled,
    ).toBe(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.Commit).toHaveBeenCalledWith('/repo', 'a message', true)
    expect(Facade.Push).not.toHaveBeenCalled()
  })
})
