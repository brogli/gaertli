import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountNotices } from '@/testing/mountNotices'
import { editorNotFound } from '@/testing/errors'
import { mountInBody } from '@/testing/mountInBody'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
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
import BranchInWorktreeDialog from '@/components/BranchInWorktreeDialog.vue'
import DiffView from '@/components/DiffView.vue'
import { useLogStore } from '@/stores/log'
import { useChangesStore } from '@/stores/changes'
import { commitRow } from '@/testing/fixtures/log-rows'
import { commitDetailsFixture } from '@/testing/fixtures/commitDetails'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { summaryFixture } from '@/testing/fixtures/summary'
import { expectIconOnly } from '@/testing/expectIconOnly'
import { findButton } from '@/testing/findButton'

function mountView(props: { path: string; distro?: string }) {
  return mount(WorktreeContent, { props })
}

describe('WorktreeContent', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
    Events.On.mockReset()
  })

  it('FR-ED-06 Open in editor failing EDITOR_NOT_FOUND shows an error notice naming the command, no banner', async () => {
    Facade.OpenFolder.mockRejectedValueOnce(editorNotFound('myed'))
    const wrapper = mountView({ path: '/repo' })
    const notice = await mountNotices()
    await flushPromises()

    await findButton(wrapper, 'Open in editor')?.trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.toolbar .error-banner').exists()).toBe(false)
  })

  it('FR-UX-09 Settings is icon-only with title and aria-label', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    expectIconOnly(wrapper.get('[data-test="open-settings"]'), 'Settings')
  })

  it('FR-UX-01 a Summary rejection renders the error banner with its message', async () => {
    Facade.Summary.mockRejectedValueOnce(
      new Error('boom', { cause: { message: 'not a git repository' } }),
    )

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.get('.error-banner').text()).toContain('not a git repository')
  })

  it('FR-REPO-04 clicking Refresh calls the Refresh binding with the worktree id', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const refreshButton = findButton(wrapper, 'Refresh')
    await refreshButton?.trigger('click')

    expect(Facade.Refresh).toHaveBeenCalledTimes(1)
    expect(Facade.Refresh).toHaveBeenCalledWith('/home/user/project')
  })

  it("ADR-9 a WSL worktree's calls carry its shell id, the UNC path, not the POSIX path it shows", async () => {
    const wrapper = mountView({ path: '/home/u/r', distro: 'D' })
    await flushPromises()

    const shellId = '\\\\wsl.localhost\\D\\home\\u\\r'
    expect(Facade.Summary).toHaveBeenCalledWith(shellId)
    expect(Facade.LoadLog).toHaveBeenCalledWith(shellId)
    expect(Facade.ListWorktrees).toHaveBeenCalledWith(shellId)
    expect(wrapper.findComponent(BranchPicker).props('distro')).toBe('D')
    expect(wrapper.get('[data-test="repo-picker"]').text()).toContain('r')
    expect(wrapper.get('[data-test="worktree-picker"]').text()).toContain('r')
  })

  it('FR-BR-08 checking out a commit warns, then confirming calls CheckoutCommit once', async () => {
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
    wrapper.unmount()
  })

  it('FR-BR-08 shows "Create branch here" while detached, opening the dialog with HEAD\'s own sha', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true, sha: 'abc123' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const createHereButton = findButton(wrapper, 'Create branch here')
    expect(createHereButton).toBeTruthy()
    await createHereButton!.trigger('click')
    await wrapper.vm.$nextTick()

    const dialog = wrapper.findComponent(CreateBranchDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({ rev: 'abc123', label: 'abc123' })

    wrapper.unmount()
  })

  it('FR-BR-08 shows no "Create branch here" button while attached to a branch', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture())
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(findButton(wrapper, 'Create branch here')).toBeUndefined()
  })

  it('FR-UX-09 "Create branch here" shows its icon', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true, sha: 'abc123' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(findButton(wrapper, 'Create branch here')!.get('.p-button-icon').classes()).toContain(
      'oi-code-branch',
    )
  })

  it('FR-BR-08 cancelling the warning never calls CheckoutCommit and closes the dialog', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(LogView).vm.$emit('checkout-commit', 'abc123')
    await flushPromises()
    await body.get('[data-test="detach-cancel"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).not.toHaveBeenCalled()
    expect(body.find('[data-test="detach-confirm"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('FR-ED-01 "Open in {name}" is labelled with the editor name and calls OpenFolder with the path', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'Beta' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const button = findButton(wrapper, 'Open in Beta')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(Facade.OpenFolder).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFolder).toHaveBeenCalledWith('/home/user/project')
  })

  it('FR-ED-05 a saved editor change relabels "Open in …" without a refresh', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'Alpha' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
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
    const wrapper = mountView(props)
    await flushPromises()

    wrapper.findComponent(LogView).vm.$emit(event, ...args)
    await flushPromises()

    expect(wrapper.findComponent(dialog).props()).toMatchObject(expectedProps)
  })

  it('FR-WT-02 "New worktree from here…" in the branch picker opens the dialog with its start', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

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

  it('FR-BR-01 "New branch from here…" in the branch picker opens the dialog at that start point', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    wrapper.findComponent(BranchPicker).vm.$emit('create-branch', { ref: 'refs/remotes/origin/x' })
    await flushPromises()

    expect(wrapper.findComponent(CreateBranchDialog).props('start')).toEqual({
      ref: 'refs/remotes/origin/x',
    })
  })

  it('FR-BR-09 a tag picked in the branch picker warns, then checks out detached', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(BranchPicker).vm.$emit('checkout-commit', 'sha-v1')
    await flushPromises()
    expect(wrapper.findComponent(DetachWarningDialog).exists()).toBe(true)
    await body.get('[data-test="detach-confirm"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).toHaveBeenCalledWith('/home/user/project', 'sha-v1')
    wrapper.unmount()
  })

  it('FR-BR-05 a branch picked that is checked out elsewhere shows the dialog with that branch and path', async () => {
    const { wrapper, body } = await mountInBody(WorktreeContent, {
      props: { path: '/home/user/project' },
    })

    wrapper.findComponent(BranchPicker).vm.$emit('checkout-elsewhere', 'side', '/repo-side')
    await flushPromises()

    const dialog = wrapper.findComponent(BranchInWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props()).toMatchObject({ branch: 'side', path: '/repo-side' })
    expect(body.get('.p-dialog').text()).toContain('side')
    wrapper.unmount()
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
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
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

  it("FR-UX-05 F5 refreshes and prevents the WebView's own reload", async () => {
    mountView({ path: '/home/user/project' })
    await flushPromises()

    const event = new KeyboardEvent('keydown', { key: 'F5', cancelable: true })
    window.dispatchEvent(event)
    await flushPromises()

    expect(Facade.Refresh).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('FR-CHG-01 a repo:changed index-scope event calls changes.load again', async () => {
    const changes = useChangesStore()
    const loadSpy = vi.spyOn(changes, 'load')

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
    expect(loadSpy).toHaveBeenCalledTimes(1)

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()

    expect(loadSpy).toHaveBeenCalledTimes(2)
    expect(loadSpy).toHaveBeenLastCalledWith('/home/user/project')
    wrapper.unmount()
  })

  it("FR-REPO-04 another window's repo:changed reloads no changes; this window's does", async () => {
    mountView({ path: '/repo' })
    await flushPromises()
    Facade.Status.mockClear()

    emit('repo:changed', { sender: 'window-2', data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()
    expect(Facade.Status).not.toHaveBeenCalled()

    emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
    await flushPromises()
    expect(Facade.Status).toHaveBeenCalledWith('/repo')
  })

  it('ADR-4 after a WSL core restart the typed commit message goes to the new core, then the window reloads', async () => {
    const reload = vi.spyOn(window.location, 'reload').mockImplementation(() => {})
    let draftSaved!: () => void
    mountView({ path: '/repo' })
    await flushPromises()
    Facade.SetMessageDraft.mockReturnValue(new Promise<void>((r) => (draftSaved = r)))
    useChangesStore().message = 'half-typed'

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
