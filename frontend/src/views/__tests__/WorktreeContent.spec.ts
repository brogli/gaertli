// Requirements: FR-BR-01, FR-BR-05, FR-BR-08, FR-BR-09, FR-CHG-01, FR-ED-01, FR-ED-05, FR-ED-06, FR-REPO-04, FR-UX-01, FR-UX-05, FR-UX-09, FR-WT-02, QA-PORT-02, QA-REL-03
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountNotices } from '@/testing/mountNotices'
import { editorNotFound, wailsError } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { ChangeScope } from '../../../bindings/github.com/brogli/gaertli/internal/core'

vi.mock(
  '../../../bindings/github.com/brogli/gaertli/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { AppService, Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import WorktreeContent from '../WorktreeContent.vue'
import CreateBranchDialog from '@/components/CreateBranchDialog.vue'
import ResetDialog from '@/components/ResetDialog.vue'
import LogView from '@/components/LogView.vue'
import CreateWorktreeDialog from '@/components/CreateWorktreeDialog.vue'
import BranchPicker from '@/components/BranchPicker.vue'
import DetachWarningDialog from '@/components/DetachWarningDialog.vue'
import BranchInUseDialog from '@/components/BranchInUseDialog.vue'
import DiffView from '@/components/DiffView.vue'
import { useLogStore } from '@/stores/log'
import { useChangesStore } from '@/stores/changes'
import { commitRow } from '@/testing/fixtures/log-rows'
import { commitDetailsFixture } from '@/testing/fixtures/commitDetails'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { summaryFixture } from '@/testing/fixtures/summary'
import { expectIconOnly } from '@/testing/expectIconOnly'
import { findButton } from '@/testing/findButton'

// mountView mounts the view on path, /home/user/project unless props say
// otherwise, and waits for its first load.
async function mountView(
  props: { path: string; distro?: string } = { path: '/home/user/project' },
) {
  const wrapper = mount(WorktreeContent, { props })
  await flushPromises()
  return wrapper
}

describe('WorktreeContent', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
    Events.On.mockReset()
  })

  it('Open in editor failing EDITOR_NOT_FOUND shows an error notice naming the command, no banner', async () => {
    Facade.OpenFolder.mockRejectedValueOnce(editorNotFound('myed'))
    const wrapper = await mountView({ path: '/repo' })
    const notice = await mountNotices()

    await findButton(wrapper, 'Open in editor')?.trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.toolbar .error-banner').exists()).toBe(false)
  })

  it('Settings is icon-only with title and aria-label', async () => {
    const wrapper = await mountView({ path: '/repo' })

    expectIconOnly(wrapper.get('[data-test="open-settings"]'), 'Settings')
  })

  it('a Summary rejection renders the error banner with its message', async () => {
    Facade.Summary.mockRejectedValueOnce(wailsError('not a git repository'))

    const wrapper = await mountView()

    expect(wrapper.get('.error-banner').text()).toContain('not a git repository')
  })

  it('clicking Refresh calls the Refresh binding with the worktree id', async () => {
    const wrapper = await mountView()

    const refreshButton = findButton(wrapper, 'Refresh')
    await refreshButton?.trigger('click')

    expect(Facade.Refresh).toHaveBeenCalledTimes(1)
    expect(Facade.Refresh).toHaveBeenCalledWith('/home/user/project')
  })

  it("a WSL worktree's calls carry its shell id, the UNC path, not the POSIX path it shows", async () => {
    const wrapper = await mountView({ path: '/home/u/r', distro: 'D' })

    const shellId = '\\\\wsl.localhost\\D\\home\\u\\r'
    expect(Facade.Summary).toHaveBeenCalledWith(shellId)
    expect(Facade.LoadLog).toHaveBeenCalledWith(shellId)
    expect(Facade.ListWorktrees).toHaveBeenCalledWith(shellId)
    expect(wrapper.findComponent(BranchPicker).props('distro')).toBe('D')
    expect(wrapper.get('[data-test="repo-picker"]').text()).toContain('r')
    expect(wrapper.get('[data-test="worktree-picker"]').text()).toContain('r')
  })

  it('checking out a commit warns, then confirming calls CheckoutCommit once', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(LogView).vm.$emit('checkout-commit', 'abc123')
    await flushPromises()
    expect(body.text()).toContain('detaches HEAD')
    await body.get('[data-test="detach-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).toHaveBeenCalledTimes(1)
    expect(Facade.CheckoutCommit).toHaveBeenCalledWith('/home/user/project', 'abc123')
  })

  it('shows "Create branch here" while detached, opening the dialog with HEAD\'s own sha', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true, sha: 'abc123' }))
    const wrapper = await mountView()

    const createHereButton = findButton(wrapper, 'Create branch here')
    expect(createHereButton).toBeTruthy()
    await createHereButton!.trigger('click')
    await wrapper.vm.$nextTick()

    const dialog = wrapper.findComponent(CreateBranchDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({ rev: 'abc123', label: 'abc123' })
  })

  it('shows no "Create branch here" button while attached to a branch', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture())
    const wrapper = await mountView()

    expect(findButton(wrapper, 'Create branch here')).toBeUndefined()
  })

  it('"Create branch here" shows its icon', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true, sha: 'abc123' }))
    const wrapper = await mountView()

    expect(findButton(wrapper, 'Create branch here')!.get('.p-button-icon').classes()).toContain(
      'oi-code-branch',
    )
  })

  it('cancelling the warning never calls CheckoutCommit and closes the dialog', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(LogView).vm.$emit('checkout-commit', 'abc123')
    await flushPromises()
    await body.get('[data-test="detach-cancel"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).not.toHaveBeenCalled()
    expect(body.find('[data-test="detach-confirm"]').exists()).toBe(false)
  })

  it('"Open in {name}" is labelled with the editor name and calls OpenFolder with the path', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'Beta' }))
    const wrapper = await mountView()

    const button = findButton(wrapper, 'Open in Beta')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(Facade.OpenFolder).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFolder).toHaveBeenCalledWith('/home/user/project')
  })

  it('a saved editor change relabels "Open in …" without a refresh', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'Alpha' }))
    const wrapper = await mountView()
    expect(findButton(wrapper, 'Open in Alpha')).toBeTruthy()

    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'Beta' }))
    emit('settings:changed')
    await flushPromises()

    expect(findButton(wrapper, 'Open in Beta')).toBeTruthy()
  })

  it.each([
    [
      'FR-BR-02, FR-LOG-11 "Reset current branch to here…" opens ResetDialog with the row\'s sha and subject',
      'reset',
      ['abc123', 'Fix the bug'],
      ResetDialog,
      { sha: 'abc123', subject: 'Fix the bug' },
      { path: '/home/user/project' },
    ],
    [
      'FR-BR-01, FR-LOG-11 "New branch from here…" opens CreateBranchDialog fixed at that commit',
      'create-branch',
      ['abc123'],
      CreateBranchDialog,
      { start: { rev: 'abc123', label: 'abc123' } },
      { path: '/home/user/project' },
    ],
    [
      'FR-WT-02, FR-LOG-11, FR-REPO-07 "Create worktree here…" opens CreateWorktreeDialog at that commit with the WSL repo path and distro',
      'create-worktree',
      ['abc123'],
      CreateWorktreeDialog,
      {
        start: { newBranch: true, start: { rev: 'abc123', label: 'abc123' } },
        repoPath: '/home/u/r',
        distro: 'Ubuntu',
      },
      { path: '/home/u/r', distro: 'Ubuntu' },
    ],
  ])('%s', async (_name, event, args, dialog, expectedProps, props) => {
    const wrapper = await mountView(props)

    wrapper.findComponent(LogView).vm.$emit(event, ...args)
    await flushPromises()

    expect(wrapper.findComponent(dialog).props()).toMatchObject(expectedProps)
  })

  it('"New worktree from here…" in the branch picker opens the dialog with its start', async () => {
    const wrapper = await mountView()

    wrapper
      .findComponent(BranchPicker)
      .vm.$emit('create-worktree', { newBranch: true, start: { ref: 'refs/remotes/origin/x' } })
    await flushPromises()

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.props('start')).toEqual({
      newBranch: true,
      start: { ref: 'refs/remotes/origin/x' },
    })
  })

  it('"New branch from here…" in the branch picker opens the dialog at that start point', async () => {
    const wrapper = await mountView()

    wrapper.findComponent(BranchPicker).vm.$emit('create-branch', { ref: 'refs/remotes/origin/x' })
    await flushPromises()

    expect(wrapper.findComponent(CreateBranchDialog).props('start')).toEqual({
      ref: 'refs/remotes/origin/x',
    })
  })

  it('a tag picked in the branch picker warns, then checks out detached', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(BranchPicker).vm.$emit('checkout-commit', 'sha-v1')
    await flushPromises()
    expect(wrapper.findComponent(DetachWarningDialog).exists()).toBe(true)
    await body.get('[data-test="detach-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).toHaveBeenCalledWith('/home/user/project', 'sha-v1')
  })

  it('a branch picked that is checked out elsewhere shows the dialog with that branch and path', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(BranchPicker).vm.$emit('checkout-elsewhere', 'side', '/repo-side')
    await flushPromises()

    const dialog = wrapper.findComponent(BranchInUseDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props()).toMatchObject({ branch: 'side', path: '/repo-side' })
    expect(body.get('.p-dialog').text()).toContain('side')
  })

  it.each([
    {
      name: 'FR-DIFF-07 a commit file',
      afterMount: () => {
        useLogStore().details = commitDetailsFixture({ files: [{ path: 'x.txt', status: 'M' }] })
      },
      file: '.commit-details li',
      diff: { kind: 'commit', sha: 'abc123', path: 'x.txt', oldPath: '' },
    },
    {
      name: 'FR-DIFF-07 a changed file',
      beforeMount: () => {
        Facade.Status.mockResolvedValue(
          changedFilesFixture({ unstaged: [{ path: 'a.txt', status: 'M' }] }),
        )
      },
      file: '.changes-body li',
      diff: { kind: 'working', staged: false, path: 'a.txt', oldPath: '' },
    },
    {
      name: 'FR-CMP-02 a compared file',
      beforeMount: () => {
        Facade.CompareCommits.mockResolvedValue([
          { path: 'a.txt', status: 'M' },
          { path: 'b.txt', status: 'A' },
        ])
        useLogStore().selection = [commitRow({ sha: 'sha-a' }), commitRow({ sha: 'sha-b' })]
      },
      file: '.compare-view .files li:nth-child(2)',
      diff: { kind: 'range', from: 'sha-a', to: 'sha-b', path: 'b.txt', oldPath: '' },
    },
  ])('$name: a click shows no diff here, a double-click opens the diff window', async (row) => {
    row.beforeMount?.()
    const wrapper = await mountView()
    row.afterMount?.()
    await flushPromises()
    const file = wrapper.get(row.file)

    await file.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)

    await file.trigger('dblclick')
    expect(AppService.OpenDiff.mock.calls).toEqual([['/home/user/project', row.diff]])
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)
  })

  it("F5 refreshes and prevents the WebView's own reload", async () => {
    await mountView()

    const event = new KeyboardEvent('keydown', { key: 'F5', cancelable: true })
    window.dispatchEvent(event)
    await flushPromises()

    expect(Facade.Refresh).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('a repo:changed index-scope event calls changes.load again', async () => {
    const changes = useChangesStore()
    const loadSpy = vi.spyOn(changes, 'load')

    await mountView()
    expect(loadSpy).toHaveBeenCalledTimes(1)

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(loadSpy).toHaveBeenCalledTimes(2)
    expect(loadSpy).toHaveBeenLastCalledWith('/home/user/project')
  })

  it("another window's repo:changed reloads no changes; this window's does", async () => {
    await mountView({ path: '/repo' })
    Facade.Status.mockClear()

    emit('repo:changed', { sender: 'window-2', data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()
    expect(Facade.Status).not.toHaveBeenCalled()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()
    expect(Facade.Status).toHaveBeenCalledWith('/repo')
  })

  it('after a WSL core restart the typed commit message goes to the new core, then the window reloads', async () => {
    const reload = vi.spyOn(window.location, 'reload').mockImplementation(() => {})
    let draftSaved!: () => void
    await mountView({ path: '/repo' })
    Facade.SetMessageDraft.mockReturnValue(new Promise<void>((r) => (draftSaved = r)))
    useChangesStore().messageDraft = 'half-typed'

    emit('core:reconnected')
    await flushPromises()

    expect(Facade.SetMessageDraft).toHaveBeenCalledWith('/repo', 'half-typed')
    expect(reload).not.toHaveBeenCalled()
    draftSaved()
    await flushPromises()
    expect(reload).toHaveBeenCalledTimes(1)
    reload.mockRestore()
  })
})
