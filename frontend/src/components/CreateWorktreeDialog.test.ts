import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockAddWorktree, mockPickFolder, mockOpenRecent, mockOpenFolderAt } = vi.hoisted(() => ({
  mockAddWorktree: vi.fn(),
  mockPickFolder: vi.fn(),
  mockOpenRecent: vi.fn(),
  mockOpenFolderAt: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { AddWorktree: mockAddWorktree, OpenFolderAt: mockOpenFolderAt },
  AppService: { PickFolder: mockPickFolder, OpenRecent: mockOpenRecent },
}))

import CreateWorktreeDialog from './CreateWorktreeDialog.vue'

describe('CreateWorktreeDialog', () => {
  let wrapper: VueWrapper | undefined

  beforeEach(() => {
    mockAddWorktree.mockReset()
    mockPickFolder.mockReset().mockResolvedValue(['', false])
    mockOpenRecent.mockReset().mockResolvedValue(undefined)
    mockOpenFolderAt.mockReset().mockResolvedValue(undefined)
  })

  // createFollowUp submits with an existing branch and a successful
  // AddWorktree (case 11's own "Given"), landing on the FR-WT-12 follow-up
  // state shared by cases 11-15.
  async function createFollowUp(): Promise<DOMWrapper<HTMLElement>> {
    mockAddWorktree.mockResolvedValue({ path: '/repo-wt' })
    wrapper = mount(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [{ name: 'feature', checkedOutIn: '' }], visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)
    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()
    return body
  }

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('FR-WT-02 an existing branch chosen submits with an empty start and newBranch false', async () => {
    mockAddWorktree.mockResolvedValue({})
    wrapper = mount(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [{ name: 'feature', checkedOutIn: '' }], visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(mockAddWorktree).toHaveBeenCalledTimes(1)
    expect(mockAddWorktree).toHaveBeenCalledWith('/repo', '/repo-wt', 'feature', '', false)
  })

  it('FR-WT-02 a new branch with a start point submits with newBranch true', async () => {
    mockAddWorktree.mockResolvedValue({})
    wrapper = mount(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [], visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('input[type="checkbox"]').setValue(true)
    await body.get('input[placeholder="Branch name"]').setValue('feature-x')
    await body.get('input[placeholder="Start point (branch or commit)"]').setValue('main')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt2')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(mockAddWorktree).toHaveBeenCalledTimes(1)
    expect(mockAddWorktree).toHaveBeenCalledWith('/repo', '/repo-wt2', 'feature-x', 'main', true)
  })

  it('FR-WT-03 a rejected InvalidName shows the error and keeps the dialog open', async () => {
    mockAddWorktree.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { message: '"-bad" is not a valid branch name', code: 'INVALID_NAME' },
      }),
    )
    wrapper = mount(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [], visible: true },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    await body.get('input[type="checkbox"]').setValue(true)
    await body.get('input[placeholder="Branch name"]').setValue('-bad')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt3')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('update:visible')).toBeUndefined()
  })

  it('FR-WT-06 a newBranch start prefills the start point, leaving name empty', async () => {
    wrapper = mount(CreateWorktreeDialog, {
      props: {
        repoPath: '/repo',
        branches: [],
        visible: true,
        start: { newBranch: true, start: 'abc123' },
      },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    expect((body.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
    expect((body.get('input[placeholder="Start point (branch or commit)"]').element as HTMLInputElement).value).toBe(
      'abc123',
    )
    expect((body.get('input[placeholder="Branch name"]').element as HTMLInputElement).value).toBe('')
  })

  it('FR-WT-06 an existing-branch start preselects that branch in the dropdown', async () => {
    wrapper = mount(CreateWorktreeDialog, {
      props: {
        repoPath: '/repo',
        branches: [{ name: 'feature', checkedOutIn: '' }],
        visible: true,
        start: { newBranch: false, branch: 'feature' },
      },
      attachTo: document.body,
    })
    await flushPromises()
    const body = new DOMWrapper(document.body)

    expect((body.get('[data-test="existing-branch"]').element as HTMLSelectElement).value).toBe('feature')
  })

  it('FR-WT-12 a successful create shows the follow-up instead of closing', async () => {
    const body = await createFollowUp()

    expect(body.text()).toContain('Open in app window')
    expect(body.text()).toContain('Open in editor')
    expect(body.text()).toContain('Both')
    expect(body.text()).toContain('Not now')
    expect(wrapper!.emitted('update:visible')).toBeUndefined()
  })

  it('FR-WT-12 "Open in app window" calls OpenRecent with the new path, then closes', async () => {
    const body = await createFollowUp()

    await body.get('[data-test="open-window"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent).toHaveBeenCalledWith('/repo-wt')
    expect(mockOpenFolderAt).not.toHaveBeenCalled()
    expect(wrapper!.emitted('update:visible')).toEqual([[false]])
  })

  it('FR-WT-12 "Open in editor" calls OpenFolderAt with the new path, then closes', async () => {
    const body = await createFollowUp()

    await body.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(mockOpenFolderAt).toHaveBeenCalledTimes(1)
    expect(mockOpenFolderAt).toHaveBeenCalledWith('/repo-wt')
    expect(mockOpenRecent).not.toHaveBeenCalled()
    expect(wrapper!.emitted('update:visible')).toEqual([[false]])
  })

  it('FR-WT-12 "Both" calls OpenRecent then OpenFolderAt, then closes', async () => {
    const body = await createFollowUp()

    await body.get('[data-test="open-both"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenFolderAt).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent.mock.invocationCallOrder[0]!).toBeLessThan(mockOpenFolderAt.mock.invocationCallOrder[0]!)
    expect(wrapper!.emitted('update:visible')).toEqual([[false]])
  })

  it('FR-WT-12 "Not now" closes without calling either binding', async () => {
    const body = await createFollowUp()

    await body.get('[data-test="not-now"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).not.toHaveBeenCalled()
    expect(mockOpenFolderAt).not.toHaveBeenCalled()
    expect(wrapper!.emitted('update:visible')).toEqual([[false]])
  })
})
