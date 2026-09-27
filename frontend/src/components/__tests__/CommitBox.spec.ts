import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    Commit: vi.fn(),
    SetMessageDraft: vi.fn(),
    CommitDetails: vi.fn(),
    Push: vi.fn(),
    PushTarget: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import CommitBox from '../CommitBox.vue'
import { useChangesStore } from '@/stores/changes'
import { useRepoStore } from '@/stores/repo'
import { useRefsStore } from '@/stores/refs'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { summaryFixture } from '@/testing/fixtures/summary'
import {
  Operation,
  type Branch,
  type Refs,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '@/testing/fixtures/branch'
import { mountInBody } from '@/testing/mountInBody'

const props = { wt: '/repo' }

function refsFixture(overrides: Partial<Branch> = {}, remotes: string[] = []): Refs {
  return {
    branches: [branchFixture(overrides)],
    remotes: remotes.map((name) => ({ name, branches: [] })),
    tags: [],
  }
}

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
    Facade.Commit.mockReset().mockResolvedValue(undefined)
    Facade.SetMessageDraft.mockReset().mockResolvedValue(undefined)
    Facade.CommitDetails.mockReset()
    Facade.Push.mockReset().mockResolvedValue('job-1')
    Facade.PushTarget.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('FR-CHG-05 the Commit button is disabled with nothing staged, amend unchecked, a non-blank message', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    expect(submitButton(wrapper).element.disabled).toBe(true)
  })

  it('FR-CHG-05 the Commit button is disabled with a staged file but a blank message', () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })

    const wrapper = mount(CommitBox, { props })

    expect(submitButton(wrapper).element.disabled).toBe(true)
  })

  it('FR-CHG-05 the Commit button counts the staged files and stays disabled for a whitespace-only message', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('   ')

    expect(submitButton(wrapper).element.disabled).toBe(true)

    await wrapper.get('textarea').setValue('m')

    expect(submitButton(wrapper).element.disabled).toBe(false)
    expect(submitButton(wrapper).text()).toBe('Commit (1)')
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

  it('FR-CHG-06 the Commit button is enabled with nothing staged when amend is checked (amend needs nothing staged)', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)
    await wrapper.get('textarea').setValue('a message')

    expect(submitButton(wrapper).element.disabled).toBe(false)
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

  it('FR-CHG-06 the Amend checkbox is disabled on an unborn HEAD', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: '' })

    const wrapper = mount(CommitBox, { props })

    expect(amendCheckbox(wrapper).element.disabled).toBe(true)
  })

  it('FR-CHG-06 the Amend checkbox is disabled while an operation (merge, rebase, …) is in progress', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123', operation: Operation.MergeOperation })

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

  it('FR-CHG-06 warns that an amended, already-pushed commit will need a force push', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main', ahead: 0 })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(wrapper.text()).toContain('already been pushed')
  })

  it("FR-CHG-06 the pushed warning is an OpenVue warn Message, like squash's", async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main', ahead: 0 })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(wrapper.get('.p-message-warn').text()).toContain('already been pushed')
  })

  it('FR-CHG-06 does not warn when the current branch is ahead of its upstream', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main', ahead: 1 })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(wrapper.text()).not.toContain('already been pushed')
  })

  it('FR-CHG-06 does not warn when the current branch has no upstream', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' })

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(wrapper.text()).not.toContain('already been pushed')
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

  it('FR-UX-05 Ctrl+K commits, same as clicking Commit', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-05 Ctrl+K with nothing staged does not commit, same gating as the button', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()

    expect(Facade.Commit).not.toHaveBeenCalled()
  })

  it('FR-CHG-09 with an upstream, checking "Push after commit" then clicking Commit calls Facade.Commit then Facade.Push', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' }, ['origin'])

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.Push).toHaveBeenCalledTimes(1)
    expect(Facade.Push).toHaveBeenCalledWith('/repo', '')
    expect(Facade.Commit.mock.invocationCallOrder[0]!).toBeLessThan(
      Facade.Push.mock.invocationCallOrder[0]!,
    )
  })

  it('FR-CHG-09 the "Push after commit" checkbox is disabled while Amend is checked', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' }, ['origin'])

    const wrapper = mount(CommitBox, { props })
    await amendCheckbox(wrapper).setValue(true)

    expect(
      wrapper.get<HTMLInputElement>('[data-test="push-after-commit"] input').element.disabled,
    ).toBe(true)
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
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: true })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.PushTarget).toHaveBeenCalledWith('/repo')
    expect(Facade.Push).toHaveBeenCalledWith('/repo', 'origin')
  })

  it('FR-CHG-09 without an upstream, committing while PushTarget cannot decide opens the dialog; Cancel leaves the commit made, unpushed', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' }, ['origin'])
    Facade.PushTarget.mockResolvedValue({ remote: 'origin', autoSetUpstream: false })

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

  it('FR-CHG-09 checking Push after commit, then Amend, then committing never pushes (amend needs a force push)', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' }, ['origin'])

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await amendCheckbox(wrapper).setValue(true) // Amend, ticked after Push after commit
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(Facade.Commit).toHaveBeenCalledTimes(1)
    expect(Facade.Commit).toHaveBeenCalledWith('/repo', 'a message', true)
    expect(Facade.Push).not.toHaveBeenCalled()
  })

  it('FR-UX-09 each checkbox is inside a label whose text names it', () => {
    const wrapper = mount(CommitBox, { props })

    const labels = wrapper.findAll('label')
    const amendLabel = labels.find((l) => l.text().includes('Amend last commit'))
    const pushLabel = labels.find((l) => l.text().includes('Push after commit'))

    expect(amendLabel?.find('input[type=checkbox]').exists()).toBe(true)
    expect(pushLabel?.find('input[type=checkbox]').exists()).toBe(true)
  })
})
