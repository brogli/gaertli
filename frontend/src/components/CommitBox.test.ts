import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCommit, mockSetMessageDraft, mockCommitDetails, mockPush } = vi.hoisted(() => ({
  mockCommit: vi.fn(),
  mockSetMessageDraft: vi.fn(),
  mockCommitDetails: vi.fn(),
  mockPush: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Commit: mockCommit, SetMessageDraft: mockSetMessageDraft, CommitDetails: mockCommitDetails, Push: mockPush },
}))

import CommitBox from './CommitBox.vue'
import { useChangesStore } from '../stores/changes'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { summaryFixture } from '../testing/fixtures/summary'
import type { Branch, Refs } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '../testing/fixtures/branch'

const props = { wt: '/repo' }

function refsFixture(overrides: Partial<Branch> = {}): Refs {
  return { branches: [branchFixture(overrides)], remotes: [], tags: [] }
}

describe('CommitBox', () => {
  // Fake timers throughout: a real 300ms draft-save debounce started in one
  // test would otherwise fire during a later one.
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    mockCommit.mockReset().mockResolvedValue(undefined)
    mockSetMessageDraft.mockReset().mockResolvedValue(undefined)
    mockCommitDetails.mockReset()
    mockPush.mockReset().mockResolvedValue('job-1')
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('FR-CHG-05 the Commit button is disabled with nothing staged, amend unchecked, a non-blank message', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    expect((wrapper.get('[data-test="commit-submit"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('FR-CHG-05 the Commit button is disabled with a staged file but a blank message', () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })

    const wrapper = mount(CommitBox, { props })

    expect((wrapper.get('[data-test="commit-submit"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('FR-CHG-05 clicking Commit calls Facade.Commit with the message, amend false', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="commit-submit"]').trigger('click')

    expect(mockCommit).toHaveBeenCalledTimes(1)
    expect(mockCommit).toHaveBeenCalledWith('/repo', 'a message', false)
  })

  it('FR-CHG-06 the Commit button is enabled with nothing staged when amend is checked (amend needs nothing staged)', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.get('textarea').setValue('a message')

    expect((wrapper.get('[data-test="commit-submit"]').element as HTMLButtonElement).disabled).toBe(false)
  })

  it('FR-CHG-05 a successful commit clears the message and resets amend', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.get('[data-test="commit-submit"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect((wrapper.get('input[type=checkbox]').element as HTMLInputElement).checked).toBe(false)
  })

  it('FR-CHG-05 the Commit button is disabled while a commit is in flight, so a double-click cannot amend twice', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    let resolveCommit: () => void = () => {}
    mockCommit.mockReturnValue(new Promise<void>((resolve) => (resolveCommit = resolve)))

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="commit-submit"]').trigger('click')

    expect((wrapper.get('[data-test="commit-submit"]').element as HTMLButtonElement).disabled).toBe(true)

    resolveCommit()
    await flushPromises()

    expect(mockCommit).toHaveBeenCalledTimes(1)
  })

  it('FR-CHG-06 the Amend checkbox is disabled on an unborn HEAD', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: '' })

    const wrapper = mount(CommitBox, { props })

    expect((wrapper.get('input[type=checkbox]').element as HTMLInputElement).disabled).toBe(true)
  })

  it('FR-CHG-06 the Amend checkbox is disabled while an operation (merge, rebase, …) is in progress', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123', operation: 'merge' })

    const wrapper = mount(CommitBox, { props })

    expect((wrapper.get('input[type=checkbox]').element as HTMLInputElement).disabled).toBe(true)
  })

  it('FR-CHG-06 checking Amend with an empty message pre-fills it from CommitDetails', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    mockCommitDetails.mockResolvedValue({ body: 'last msg' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true)
    await flushPromises()

    expect(mockCommitDetails).toHaveBeenCalledTimes(1)
    expect(mockCommitDetails).toHaveBeenCalledWith('/repo', 'abc123')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('last msg')
  })

  it('FR-CHG-06 checking Amend never clobbers a message already typed', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('typed draft')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await flushPromises()

    expect(mockCommitDetails).not.toHaveBeenCalled()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('typed draft')
  })

  it('FR-CHG-06 warns that an amended, already-pushed commit will need a force push', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main', ahead: 0 })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true)

    expect(wrapper.text()).toContain('already been pushed')
  })

  it('FR-CHG-06 does not warn when the current branch is ahead of its upstream', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main', ahead: 1 })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true)

    expect(wrapper.text()).not.toContain('already been pushed')
  })

  it('FR-CHG-06 does not warn when the current branch has no upstream', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true)

    expect(wrapper.text()).not.toContain('already been pushed')
  })

  it('FR-CHG-08 initializes the message field from the restored draft', () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ messageDraft: 'restored' })

    const wrapper = mount(CommitBox, { props })

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('restored')
  })

  it('FR-CHG-08 saves the message as a draft, debounced 300ms', async () => {
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('typing...')
    await vi.advanceTimersByTimeAsync(300)

    expect(mockSetMessageDraft).toHaveBeenCalledTimes(1)
    expect(mockSetMessageDraft).toHaveBeenCalledWith('/repo', 'typing...')
  })

  it('FR-CHG-08 does not re-save a restored draft that was never edited', async () => {
    const repo = useRepoStore()
    const wrapper = mount(CommitBox, { props })
    // Summary (and its draft) arrives asynchronously, after CommitBox has
    // already mounted and is watching `message` for edits to save.
    repo.summary = summaryFixture({ messageDraft: 'restored' })
    await wrapper.vm.$nextTick()
    await vi.advanceTimersByTimeAsync(300)

    expect(mockSetMessageDraft).not.toHaveBeenCalled()
  })

  it('FR-UX-05 Ctrl+K commits, same as clicking Commit', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()

    expect(mockCommit).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-05 Ctrl+K with nothing staged does not commit, same gating as the button', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture()
    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()

    expect(mockCommit).not.toHaveBeenCalled()
  })

  it('FR-CHG-09 with an upstream, checking "Push after commit" then clicking Commit calls Facade.Commit then Facade.Push', async () => {
    const changes = useChangesStore()
    changes.files = changedFilesFixture({ staged: [{ path: 'a.txt', status: 'M' }] })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await wrapper.get('[data-test="commit-submit"]').trigger('click')
    await flushPromises()

    expect(mockCommit).toHaveBeenCalledTimes(1)
    expect(mockPush).toHaveBeenCalledTimes(1)
    expect(mockPush).toHaveBeenCalledWith('/repo', '')
    expect(mockCommit.mock.invocationCallOrder[0]!).toBeLessThan(mockPush.mock.invocationCallOrder[0]!)
  })

  it('FR-CHG-09 the "Push after commit" checkbox is disabled while Amend is checked', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('input[type=checkbox]').setValue(true) // Amend

    expect((wrapper.get('[data-test="push-after-commit"] input').element as HTMLInputElement).disabled).toBe(true)
  })

  it('FR-CHG-09 the "Push after commit" checkbox is disabled with no upstream', () => {
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: '' })

    const wrapper = mount(CommitBox, { props })

    expect((wrapper.get('[data-test="push-after-commit"] input').element as HTMLInputElement).disabled).toBe(true)
  })

  it('FR-CHG-09 checking Push after commit, then Amend, then committing never pushes (amend needs a force push)', async () => {
    const repo = useRepoStore()
    repo.summary = summaryFixture({ sha: 'abc123' })
    const refs = useRefsStore()
    refs.refs = refsFixture({ upstream: 'origin/main' })

    const wrapper = mount(CommitBox, { props })
    await wrapper.get('textarea').setValue('a message')
    await wrapper.get('[data-test="push-after-commit"] input').setValue(true)
    await wrapper.get('input[type=checkbox]').setValue(true) // Amend, ticked after Push after commit
    await wrapper.get('[data-test="commit-submit"]').trigger('click')
    await flushPromises()

    expect(mockCommit).toHaveBeenCalledTimes(1)
    expect(mockCommit).toHaveBeenCalledWith('/repo', 'a message', true)
    expect(mockPush).not.toHaveBeenCalled()
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
