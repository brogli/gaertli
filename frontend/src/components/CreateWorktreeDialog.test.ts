import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
import { mountInBody } from '../testing/mountInBody'

describe('CreateWorktreeDialog', () => {
  beforeEach(() => {
    mockAddWorktree.mockReset()
    mockPickFolder.mockReset().mockResolvedValue(['', false])
    mockOpenRecent.mockReset().mockResolvedValue(undefined)
    mockOpenFolderAt.mockReset().mockResolvedValue(undefined)
  })

  // createFollowUp submits with an existing branch and a successful
  // AddWorktree, landing on the "open the result" follow-up.
  async function createFollowUp() {
    mockAddWorktree.mockResolvedValue({ path: '/repo-wt' })
    const mounted = await mountInBody(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [{ name: 'feature', checkedOutIn: '' }] },
    })
    const { body } = mounted
    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()
    return mounted
  }

  it('FR-WT-02 an existing branch chosen submits with an empty start and newBranch false', async () => {
    mockAddWorktree.mockResolvedValue({})
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [{ name: 'feature', checkedOutIn: '' }] },
    })

    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(mockAddWorktree).toHaveBeenCalledTimes(1)
    expect(mockAddWorktree).toHaveBeenCalledWith('/repo', '/repo-wt', 'feature', '', false)
  })

  it('FR-WT-02 a new branch with a start point submits with newBranch true', async () => {
    mockAddWorktree.mockResolvedValue({})
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [] },
    })

    await body.get('input[type="checkbox"]').setValue(true)
    await body.get('input[placeholder="Branch name"]').setValue('feature-x')
    await body.get('input[placeholder="Start point (branch or commit)"]').setValue('main')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt2')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(mockAddWorktree).toHaveBeenCalledTimes(1)
    expect(mockAddWorktree).toHaveBeenCalledWith('/repo', '/repo-wt2', 'feature-x', 'main', true)
  })

  it('FR-REPO-07 a WSL repo submits AddWorktree with the UNC repo and target paths', async () => {
    mockAddWorktree.mockResolvedValue({})
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: {
        repoPath: '/home/u/r',
        distro: 'Ubuntu',
        branches: [{ name: 'feature', checkedOutIn: '' }],
      },
    })

    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/home/u/r/wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(mockAddWorktree).toHaveBeenCalledWith(
      '\\\\wsl.localhost\\Ubuntu\\home\\u\\r',
      '\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt',
      'feature',
      '',
      false,
    )
  })

  it("FR-REPO-07 Browse seeds PickFolder at the distro's UNC root for a WSL repo", async () => {
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: { repoPath: '/home/u/r', distro: 'Ubuntu', branches: [] },
    })

    await body.get('[data-test="browse-target"]').trigger('click')

    expect(mockPickFolder).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu')
  })

  // wslFollowUp submits a WSL repo's dialog to its "open the result" follow-up.
  async function wslFollowUp() {
    mockAddWorktree.mockResolvedValue({ path: '/home/u/r/wt' })
    const mounted = await mountInBody(CreateWorktreeDialog, {
      props: {
        repoPath: '/home/u/r',
        distro: 'Ubuntu',
        branches: [{ name: 'feature', checkedOutIn: '' }],
      },
    })
    const { body } = mounted
    await body.get('[data-test="existing-branch"]').setValue('feature')
    await body.get('input[placeholder="Target folder"]').setValue('/home/u/r/wt')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()
    return mounted
  }

  it('FR-REPO-07 "Open in app window" for a WSL repo calls OpenRecent with the UNC form of the created path', async () => {
    const { body } = await wslFollowUp()

    await body.get('[data-test="open-window"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt')
    expect(mockOpenFolderAt).not.toHaveBeenCalled()
  })

  it('FR-REPO-07 "Open in editor" for a WSL repo calls OpenFolderAt with the UNC form of the created path', async () => {
    const { body } = await wslFollowUp()

    await body.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(mockOpenFolderAt).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt')
    expect(mockOpenRecent).not.toHaveBeenCalled()
  })

  it('FR-WT-03 a rejected InvalidName shows the error and keeps the dialog open', async () => {
    mockAddWorktree.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { message: '"-bad" is not a valid branch name', code: 'INVALID_NAME' },
      }),
    )
    const { wrapper, body } = await mountInBody(CreateWorktreeDialog, {
      props: { repoPath: '/repo', branches: [] },
    })

    await body.get('input[type="checkbox"]').setValue(true)
    await body.get('input[placeholder="Branch name"]').setValue('-bad')
    await body.get('input[placeholder="Target folder"]').setValue('/repo-wt3')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-WT-06 a newBranch start prefills the start point, leaving name empty', async () => {
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: {
        repoPath: '/repo',
        branches: [],
        start: { newBranch: true, start: 'abc123' },
      },
    })

    expect((body.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
    expect((body.get('input[placeholder="Start point (branch or commit)"]').element as HTMLInputElement).value).toBe(
      'abc123',
    )
    expect((body.get('input[placeholder="Branch name"]').element as HTMLInputElement).value).toBe('')
  })

  it('FR-WT-06 an existing-branch start preselects that branch in the dropdown', async () => {
    const { body } = await mountInBody(CreateWorktreeDialog, {
      props: {
        repoPath: '/repo',
        branches: [{ name: 'feature', checkedOutIn: '' }],
        start: { newBranch: false, branch: 'feature' },
      },
    })

    expect((body.get('[data-test="existing-branch"]').element as HTMLSelectElement).value).toBe('feature')
  })

  it('FR-WT-12 a successful create shows the follow-up instead of closing', async () => {
    const { wrapper, body } = await createFollowUp()

    expect(body.text()).toContain('Open in app window')
    expect(body.text()).toContain('Open in editor')
    expect(body.text()).toContain('Both')
    expect(body.text()).toContain('Not now')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-WT-12 "Open in app window" calls OpenRecent with the new path, then closes', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="open-window"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent).toHaveBeenCalledWith('/repo-wt')
    expect(mockOpenFolderAt).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-WT-12 "Open in editor" calls OpenFolderAt with the new path, then closes', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(mockOpenFolderAt).toHaveBeenCalledTimes(1)
    expect(mockOpenFolderAt).toHaveBeenCalledWith('/repo-wt')
    expect(mockOpenRecent).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-WT-12 "Both" calls OpenRecent then OpenFolderAt, then closes', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="open-both"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).toHaveBeenCalledTimes(1)
    expect(mockOpenFolderAt).toHaveBeenCalledTimes(1)
    expect(mockOpenRecent.mock.invocationCallOrder[0]!).toBeLessThan(mockOpenFolderAt.mock.invocationCallOrder[0]!)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-WT-12 "Not now" closes without calling either binding', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="not-now"]').trigger('click')
    await flushPromises()

    expect(mockOpenRecent).not.toHaveBeenCalled()
    expect(mockOpenFolderAt).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
