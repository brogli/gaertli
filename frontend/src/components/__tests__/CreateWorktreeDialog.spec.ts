import { type DOMWrapper, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: { AddWorktree: vi.fn(), OpenFolderAt: vi.fn(), ListWorktrees: vi.fn() },
  AppService: { PickFolder: vi.fn(), OpenPath: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import CreateWorktreeDialog from '../CreateWorktreeDialog.vue'
import { mountInBody } from '@/testing/mountInBody'
import { wailsError } from '@/testing/errors'
import { chooseOption } from '@/testing/select'
import { field, fieldLabels } from '@/testing/dialog'
import { branchFixture } from '@/testing/fixtures/branch'
import { refsOfEveryKind } from '@/testing/fixtures/refs'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { worktreeFixture } from '@/testing/fixtures/worktree'
import { useRefsStore } from '@/stores/refs'
import { useSettingsStore } from '@/stores/settings'
import type { WorktreeStart } from '../startPoint'

const wslRepo = { repoPath: '/home/u/r', distro: 'Ubuntu' }

// mountDialog mounts the dialog on a repo whose branches are main (current)
// and feature, with the remote branch origin/x and the tag v1.
function mountDialog(props: { repoPath: string; distro?: string; start?: WorktreeStart }) {
  useRefsStore().refs = refsOfEveryKind([
    branchFixture(),
    branchFixture({ name: 'feature', current: false }),
  ])
  return mountInBody(CreateWorktreeDialog, { props })
}

// submitExisting mounts the dialog on props and submits the existing branch
// "feature" into target.
async function submitExisting(props: { repoPath: string; distro?: string }, target: string) {
  const mounted = await mountDialog(props)
  await chooseOption(mounted.body, 'existing-branch', 'feature')
  await field(mounted.body, 'Location').setValue(target)
  await mounted.body.get('[data-test="create-worktree-submit"]').trigger('click')
  await flushPromises()
  return mounted
}

describe('CreateWorktreeDialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.AddWorktree.mockReset()
    Facade.ListWorktrees.mockReset().mockResolvedValue([
      worktreeFixture({ path: '/repos/alpha', branch: 'main', main: true }),
      worktreeFixture({ path: '/repos/alpha-feature', branch: 'feature' }),
    ])
    AppService.PickFolder.mockReset().mockResolvedValue(['', false])
    AppService.OpenPath.mockReset().mockResolvedValue(undefined)
    Facade.OpenFolderAt.mockReset().mockResolvedValue(undefined)
  })

  // createFollowUp submits with an existing branch and a successful
  // AddWorktree, landing on the "open the result" follow-up.
  function createFollowUp() {
    Facade.AddWorktree.mockResolvedValue({ path: '/repo-wt' })
    return submitExisting({ repoPath: '/repo' }, '/repo-wt')
  }

  it.each([
    { env: 'local', props: { repoPath: '/repo' }, repo: '/repo', target: '/repo-wt' },
    {
      env: 'WSL',
      props: wslRepo,
      repo: '\\\\wsl.localhost\\Ubuntu\\home\\u\\r',
      target: '\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt',
    },
  ])(
    'FR-WT-02 an existing branch chosen submits with an empty start and newBranch false ($env repo, WSL paths as UNC)',
    async ({ props, repo, target }) => {
      Facade.AddWorktree.mockResolvedValue({})

      await submitExisting(props, props === wslRepo ? '/home/u/r/wt' : '/repo-wt')

      expect(Facade.AddWorktree).toHaveBeenCalledTimes(1)
      expect(Facade.AddWorktree).toHaveBeenCalledWith(repo, target, 'feature', '', false)
    },
  )

  // fillNew checks "New branch" unless the dialog was opened on a new-branch
  // start, names the branch feature-x, picks the start branch pickStart if
  // given, then submits with the default location.
  async function fillNew(body: DOMWrapper<Element>, pickStart?: string, checkNew = true) {
    Facade.AddWorktree.mockResolvedValue({})
    if (checkNew) await body.get('input[type="checkbox"]').setValue(true)
    await field(body, 'Branch name').setValue('feature-x')
    if (pickStart) await chooseOption(body, 'start-branch', pickStart)
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()
  }

  it.each([
    { start: undefined, ref: 'refs/heads/main' },
    { start: 'origin/x', ref: 'refs/remotes/origin/x' },
    { start: 'v1', ref: 'refs/tags/v1' },
  ])(
    'FR-WT-02 a new branch starts at the current branch unless a branch or tag is picked, sent by its full ref name; the location defaults to the base folder ($start)',
    async ({ start, ref }) => {
      const { body } = await mountDialog({ repoPath: '/repos/alpha' })

      await fillNew(body, start)

      expect(Facade.AddWorktree).toHaveBeenCalledWith(
        '/repos/alpha',
        '/repos/alpha-feature-x',
        'feature-x',
        ref,
        true,
      )
    },
  )

  it('FR-WT-02 a new branch from the log view starts at that commit, shown fixed', async () => {
    const start: WorktreeStart = { newBranch: true, start: { rev: 'abc123456', label: 'abc1234' } }
    const { body } = await mountDialog({ repoPath: '/repos/alpha', start })
    expect(field(body, 'Start point').element.value).toBe('abc1234')
    expect(field(body, 'Start point').element.disabled).toBe(true)

    await fillNew(body, undefined, false)

    expect(Facade.AddWorktree).toHaveBeenCalledWith(
      '/repos/alpha',
      '/repos/alpha-feature-x',
      'feature-x',
      'abc123456',
      true,
    )
  })

  // folderFields are the dialog's Folder name and Location values.
  function folderFields(body: DOMWrapper<Element>) {
    return {
      name: field(body, 'Folder name').element.value,
      location: field(body, 'Location').element.value,
    }
  }

  it("FR-WT-02 unset in Settings, the base folder is the main worktree's parent; the name joins the repo's folder and the branch", async () => {
    const { body } = await mountDialog({ repoPath: '/repos/alpha-feature' })

    await chooseOption(body, 'existing-branch', 'feature')

    expect(Facade.ListWorktrees).toHaveBeenCalledWith('/repos/alpha-feature')
    expect(folderFields(body)).toEqual({ name: 'alpha-feature', location: '/repos/alpha-feature' })
  })

  it("FR-WT-02 Settings' base folder for the repo's environment holds the new worktree", async () => {
    useSettingsStore().settings = defaultSettingsFixture({
      worktreeFolders: { '': '/wt', Ubuntu: '/home/u/wt' },
    })

    const local = await mountDialog({ repoPath: '/repos/alpha' })
    await chooseOption(local.body, 'existing-branch', 'feature')
    expect(folderFields(local.body).location).toBe('/wt/alpha-feature')
    local.wrapper.unmount()

    const wsl = await mountDialog(wslRepo)
    await chooseOption(wsl.body, 'existing-branch', 'feature')
    expect(folderFields(wsl.body).location).toBe('/home/u/wt/alpha-feature')
  })

  it("FR-WT-02 the name and the location's last folder stay in step", async () => {
    const { body } = await mountDialog({ repoPath: '/repos/alpha' })

    await field(body, 'Folder name').setValue('x')
    expect(folderFields(body)).toEqual({ name: 'x', location: '/repos/x' })

    await field(body, 'Location').setValue('/other/y')
    expect(folderFields(body)).toEqual({ name: 'y', location: '/other/y' })
  })

  it("FR-WT-02 the name follows a new branch's name, a slash as a hyphen, until the name is edited", async () => {
    const { body } = await mountDialog({ repoPath: '/repos/alpha' })
    await body.get('input[type="checkbox"]').setValue(true)

    await field(body, 'Branch name').setValue('feat/x')
    expect(folderFields(body)).toEqual({ name: 'alpha-feat-x', location: '/repos/alpha-feat-x' })

    await field(body, 'Folder name').setValue('mine')
    await field(body, 'Branch name').setValue('other')
    expect(folderFields(body)).toEqual({ name: 'mine', location: '/repos/mine' })
  })

  it('FR-WT-02 a long location scrolls to its end, so the changing folder name stays visible; the full path is its tooltip', async () => {
    Facade.ListWorktrees.mockResolvedValue([
      worktreeFixture({
        path: '/home/user/projects/very/deeply/nested/repositories/alpha',
        branch: 'main',
        main: true,
      }),
    ])
    const { body } = await mountDialog({ repoPath: '/repos/alpha' })
    await flushPromises()

    const location = field(body, 'Location')
    expect(location.element.selectionStart).toBe(location.element.value.length)
    expect(location.attributes('title')).toBe(location.element.value)
  })

  it('FR-WT-02 a folder picked with Browse… becomes the location, its last folder the name', async () => {
    AppService.PickFolder.mockResolvedValue(['/picked/z', true])
    const { body } = await mountDialog({ repoPath: '/repos/alpha' })

    await body.get('[data-test="browse-target"]').trigger('click')
    await flushPromises()

    expect(folderFields(body)).toEqual({ name: 'z', location: '/picked/z' })
  })

  it('FR-UX-17 a new branch adds its own labelled fields; Browse sits beside Location', async () => {
    const { body } = await mountDialog({ repoPath: '/repo' })

    await body.get('input[type="checkbox"]').setValue(true)
    expect(fieldLabels(body)).toEqual(['Branch name', 'Start point', 'Folder name', 'Location'])
    expect(body.find('.p-inputgroup [data-test="browse-target"]').exists()).toBe(true)
  })

  it("FR-REPO-07 Browse seeds PickFolder at the distro's UNC root for a WSL repo", async () => {
    const { body } = await mountDialog(wslRepo)

    await body.get('[data-test="browse-target"]').trigger('click')

    expect(AppService.PickFolder).toHaveBeenCalledWith('\\\\wsl.localhost\\Ubuntu')
  })

  it('FR-WT-03 a rejected InvalidName shows the error and keeps the dialog open', async () => {
    Facade.AddWorktree.mockRejectedValueOnce(
      wailsError('"-bad" is not a valid branch name', 'INVALID_NAME'),
    )
    const { wrapper, body } = await mountDialog({ repoPath: '/repo' })

    await body.get('input[type="checkbox"]').setValue(true)
    await field(body, 'Branch name').setValue('-bad')
    await field(body, 'Location').setValue('/repo-wt3')
    await body.get('[data-test="create-worktree-submit"]').trigger('click')
    await flushPromises()

    expect(body.text()).toContain('is not a valid branch name')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('FR-WT-02 an existing-branch start preselects that branch in the dropdown', async () => {
    const { body } = await mountDialog({
      repoPath: '/repo',
      start: { newBranch: false, branch: 'feature' },
    })

    expect(body.get('[data-test="existing-branch"]').text()).toBe('feature')
  })

  it('FR-WT-12 a successful create shows the follow-up instead of closing', async () => {
    const { wrapper, body } = await createFollowUp()

    expect(body.text()).toContain('Open in app window')
    expect(body.text()).toContain('Open in editor')
    expect(body.text()).toContain('Both')
    expect(body.text()).toContain('Not now')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  const followUps = [
    { env: 'local', props: { repoPath: '/repo' }, created: '/repo-wt', opened: '/repo-wt' },
    {
      env: 'WSL',
      props: wslRepo,
      created: '/home/u/r/wt',
      opened: '\\\\wsl.localhost\\Ubuntu\\home\\u\\r\\wt',
    },
  ].flatMap((repo) => [
    { ...repo, button: 'open-window', called: AppService.OpenPath, other: Facade.OpenFolderAt },
    { ...repo, button: 'open-editor', called: Facade.OpenFolderAt, other: AppService.OpenPath },
  ])

  it.each(followUps)(
    'FR-WT-12 $button opens the created path, as its UNC form in a WSL repo, then closes ($env)',
    async ({ props, created, opened, button, called, other }) => {
      Facade.AddWorktree.mockResolvedValue({ path: created })
      const { wrapper, body } = await submitExisting(props, created)

      await body.get(`[data-test="${button}"]`).trigger('click')
      await flushPromises()

      expect(called).toHaveBeenCalledTimes(1)
      expect(called).toHaveBeenCalledWith(opened)
      expect(other).not.toHaveBeenCalled()
      expect(wrapper.emitted('close')).toHaveLength(1)
    },
  )

  it('FR-WT-12 "Both" calls OpenPath then OpenFolderAt, then closes', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="open-both"]').trigger('click')
    await flushPromises()

    expect(AppService.OpenPath).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFolderAt).toHaveBeenCalledTimes(1)
    expect(AppService.OpenPath.mock.invocationCallOrder[0]!).toBeLessThan(
      Facade.OpenFolderAt.mock.invocationCallOrder[0]!,
    )
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('FR-WT-12 "Not now" closes without calling either binding', async () => {
    const { wrapper, body } = await createFollowUp()

    await body.get('[data-test="not-now"]').trigger('click')
    await flushPromises()

    expect(AppService.OpenPath).not.toHaveBeenCalled()
    expect(Facade.OpenFolderAt).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
