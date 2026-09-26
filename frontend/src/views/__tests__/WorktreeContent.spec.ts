import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '@/testing/mountInBody'
import NoticeArea from '@/components/NoticeArea.vue'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { AppService, Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import SplitButton from 'openvue/splitbutton'
import WorktreeContent from '../WorktreeContent.vue'
import CreateBranchDialog from '@/components/CreateBranchDialog.vue'
import ResetDialog from '@/components/ResetDialog.vue'
import LogView from '@/components/LogView.vue'
import CommitDetailView from '@/components/CommitDetailView.vue'
import CreateWorktreeDialog from '@/components/CreateWorktreeDialog.vue'
import ChangesView from '@/components/ChangesView.vue'
import BranchPicker from '@/components/BranchPicker.vue'
import RepoPicker from '@/components/RepoPicker.vue'
import WorktreePicker from '@/components/WorktreePicker.vue'
import DetachWarningDialog from '@/components/DetachWarningDialog.vue'
import BranchInWorktreeDialog from '@/components/BranchInWorktreeDialog.vue'
import DiffView from '@/components/DiffView.vue'
import SyncControls from '@/components/SyncControls.vue'
import { useLogStore } from '@/stores/log'
import { useChangesStore } from '@/stores/changes'
import { commitRow } from '@/testing/fixtures/log-rows'
import { commitDetailsFixture } from '@/testing/fixtures/commitDetails'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { branchFixture } from '@/testing/fixtures/branch'
import { summaryFixture } from '@/testing/fixtures/summary'
import { findButton } from '@/testing/findButton'
import { clickMenuItem } from '@/testing/contextMenu'

function mountView(props: { path: string; distro?: string }) {
  return mount(WorktreeContent, { props })
}

const sampleRow = commitRow()

// mountAndPickRowMenuItem mounts WorktreeContent with one log row,
// right-clicks it, and picks label from the resulting menu.
async function mountAndPickRowMenuItem(
  label: string,
  props: { path: string; distro?: string } = { path: '/home/user/project' },
) {
  const { wrapper, body } = await mountInBody(WorktreeContent, { props })
  const log = useLogStore()
  log.rowCache.set(0, sampleRow)
  log.loaded = 1
  await wrapper.vm.$nextTick()

  await wrapper.get('.row').trigger('contextmenu')
  await wrapper.vm.$nextTick()
  await clickMenuItem(body, label)
  await flushPromises()

  return { wrapper, body }
}

describe('WorktreeContent', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
    Events.On.mockReset()
  })

  it('FR-UX-08 at the default (medium) width, the log sits beside a side panel showing Details by default', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    const [outer] = wrapper.findAllComponents(Splitter)
    expect(outer!.props('layout')).toBe('horizontal')
    const [logPanel, sidePanel] = outer!.findAllComponents(SplitterPanel)
    expect(logPanel!.findComponent(LogView).exists()).toBe(true)
    expect(sidePanel!.findComponent(CommitDetailView).exists()).toBe(true)
    expect(sidePanel!.findComponent(ChangesView).exists()).toBe(true) // mounted, hidden
  })

  it('FR-UX-02 renders no per-command git list: every command goes to the app log instead', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    expect(wrapper.find('.command-log').exists()).toBe(false)
  })

  it('FR-UX-07 the toolbar holds the repo, worktree and branch pickers, the sync controls, Refresh, Open in editor and the gear', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    const toolbar = wrapper.get('.toolbar')
    expect(toolbar.findComponent(RepoPicker).props('path')).toBe('/repo')
    expect(toolbar.findComponent(WorktreePicker).props('path')).toBe('/repo')
    expect(toolbar.findComponent(BranchPicker).props('wt')).toBe('/repo')
    expect(toolbar.findComponent(SyncControls).exists()).toBe(true)
    const buttonNames = toolbar.findAll('button').map((b) => b.attributes('aria-label') || b.text())
    expect(buttonNames).toEqual(expect.arrayContaining(['Refresh', 'Open in editor']))
  })

  it('FR-ED-06 Open in editor failing EDITOR_NOT_FOUND shows an error notice naming the command, no banner', async () => {
    Facade.OpenFolder.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'EDITOR_NOT_FOUND', message: '"myed" is not on PATH' } }),
    )
    const wrapper = mountView({ path: '/repo' })
    const { body: notice } = await mountInBody(NoticeArea)
    await flushPromises()

    await findButton(wrapper, 'Open in editor')?.trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.toolbar .error-banner').exists()).toBe(false)
  })

  it('FR-WT-13 the toolbar shows the repo, worktree and branch pickers in that order', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    const buttons = ['repo-picker', 'worktree-picker', 'branch-picker']
    const order = wrapper
      .get('.toolbar')
      .findAll('button')
      .map((b) => b.attributes('data-test'))
      .filter((t) => buttons.includes(t ?? ''))
    expect(order).toEqual(buttons)
  })

  it('TR-34 the toolbar buttons render their icon; Settings is icon-only with title and aria-label', async () => {
    Facade.Refs.mockResolvedValue({
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    })
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    const iconOf = (label: string) => findButton(wrapper, label)!.get('.p-button-icon').classes()
    expect(iconOf('Fetch')).toContain('oi-cloud-download')
    expect(wrapper.findComponent(SplitButton).get('i').classes()).toContain('oi-download')
    expect(iconOf('Push')).toContain('oi-upload')
    expect(iconOf('Refresh')).toContain('oi-refresh')
    expect(iconOf('Open in editor')).toContain('oi-external-link')

    const settings = wrapper.get('[data-test="open-settings"]')
    expect(settings.attributes('title')).toBe('Settings')
    expect(settings.attributes('aria-label')).toBe('Settings')
    expect(settings.get('.p-button-icon').classes()).toContain('oi-cog')
  })

  it('FR-UX-04 the gear asks for the Settings', async () => {
    const wrapper = mountView({ path: '/repo' })
    await flushPromises()

    await wrapper.get('.toolbar [data-test="open-settings"]').trigger('click')

    expect(wrapper.emitted('open-settings')).toHaveLength(1)
  })

  it('FR-WT-09 shows the worktree name from the path prop', () => {
    expect(mountView({ path: '/home/user/project' }).text()).toContain('project')
  })

  it('FR-UX-01 shows the error message and reveals gitStderr behind show details', async () => {
    Facade.Summary.mockRejectedValueOnce(
      new Error('boom', {
        cause: { message: 'not a git repository', gitStderr: 'fatal: not a git repository' },
      }),
    )

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.text()).toContain('not a git repository')
    expect(wrapper.text()).not.toContain('fatal: not a git repository')

    await wrapper.get('.error-banner button').trigger('click')

    expect(wrapper.text()).toContain('fatal: not a git repository')
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

  it('FR-REPO-03 shows an ahead/behind indicator when the branch has an upstream', async () => {
    Facade.Refs.mockResolvedValue({
      branches: [branchFixture({ upstream: 'origin/main', ahead: 2, behind: 1 })],
      remotes: [],
      tags: [],
    })

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const indicator = wrapper.get('[data-test="ahead-behind"]')
    expect(indicator.text()).toContain('2')
    expect(indicator.text()).toContain('1')
  })

  it('FR-REPO-03 shows no ahead/behind indicator without an upstream', async () => {
    Facade.Refs.mockResolvedValue({ branches: [branchFixture()], remotes: [], tags: [] })

    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })

  it('FR-SYNC-01, FR-SYNC-02, FR-SYNC-03 renders SyncControls in the summary area', async () => {
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    expect(wrapper.findComponent(SyncControls).exists()).toBe(true)
  })

  it('FR-BR-08 checking out a commit warns, then confirming calls CheckoutCommit once', async () => {
    const { wrapper, body } = await mountAndPickRowMenuItem('Check out commit')

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

  it('FR-UX-09 "Create branch here" is an OpenVue Button, secondary and outlined, with its icon', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true, sha: 'abc123' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const button = findButton(wrapper, 'Create branch here')!
    expect(button.classes()).toContain('p-button-secondary')
    expect(button.classes()).toContain('p-button-outlined')
    expect(button.get('.p-button-icon').classes()).toContain('oi-code-branch')
  })

  it('FR-BR-08 cancelling the warning never calls CheckoutCommit and closes the dialog', async () => {
    const { wrapper, body } = await mountAndPickRowMenuItem('Check out commit')

    await body.get('[data-test="detach-cancel"]').trigger('click')
    await flushPromises()

    expect(Facade.CheckoutCommit).not.toHaveBeenCalled()
    expect(body.find('[data-test="detach-confirm"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('FR-BR-02, FR-LOG-11 "Reset current branch to here…" opens ResetDialog with the row\'s sha and subject', async () => {
    const { wrapper } = await mountAndPickRowMenuItem('Reset current branch to here…')

    const dialog = wrapper.findComponent(ResetDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('sha')).toBe('abc123')
    expect(dialog.props('subject')).toBe('Fix the bug')

    wrapper.unmount()
  })

  it('FR-ED-01 "Open in {name}" is labelled with the editor name and calls OpenFolder with the path', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'another editor' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()

    const button = findButton(wrapper, 'Open in another editor')
    expect(button).toBeTruthy()

    await button!.trigger('click')

    expect(Facade.OpenFolder).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFolder).toHaveBeenCalledWith('/home/user/project')
  })

  it('FR-ED-05 a saved editor change relabels "Open in …" without a refresh', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'the editor' }))
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
    expect(findButton(wrapper, 'Open in the editor')).toBeTruthy()

    Facade.Summary.mockResolvedValue(summaryFixture({ editorName: 'another editor' }))
    emit('settings:changed')
    await flushPromises()

    expect(findButton(wrapper, 'Open in another editor')).toBeTruthy()
  })

  it('FR-BR-01, FR-LOG-11 "New branch from here…" from the log opens the dialog fixed at that commit', async () => {
    const { wrapper } = await mountAndPickRowMenuItem('New branch from here…')

    const dialog = wrapper.findComponent(CreateBranchDialog)
    expect(dialog.props('start')).toEqual({ rev: 'abc123', label: 'abc123' })

    wrapper.unmount()
  })

  it('FR-WT-06, FR-LOG-11 "Create worktree here…" from the log opens the dialog pre-filled at that commit', async () => {
    const { wrapper } = await mountAndPickRowMenuItem('Create worktree here…')

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.exists()).toBe(true)
    expect(dialog.props('start')).toEqual({
      newBranch: true,
      start: { rev: 'abc123', label: 'abc123' },
    })

    wrapper.unmount()
  })

  it('FR-REPO-07 a WSL worktree\'s "Create worktree here…" opens the dialog with repo path and distro', async () => {
    const { wrapper } = await mountAndPickRowMenuItem('Create worktree here…', {
      path: '/home/u/r',
      distro: 'Ubuntu',
    })

    const dialog = wrapper.findComponent(CreateWorktreeDialog)
    expect(dialog.props('repoPath')).toBe('/home/u/r')
    expect(dialog.props('distro')).toBe('Ubuntu')

    wrapper.unmount()
  })

  it('FR-WT-06 "New worktree from here…" in the branch picker opens the dialog with its start', async () => {
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

  it('FR-DIFF-07 the main window shows no diff: a commit file and a changed file open in the diff window', async () => {
    Facade.Status.mockResolvedValue(
      changedFilesFixture({ unstaged: [{ path: 'a.txt', status: 'M' }] }),
    )
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
    useLogStore().details = commitDetailsFixture({ files: [{ path: 'x.txt', status: 'M' }] })
    await flushPromises()
    const commitFile = wrapper.get('.commit-details li')
    const changedFile = wrapper.get('.changes-body li')

    await commitFile.trigger('click')
    await changedFile.trigger('click')
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)

    await commitFile.trigger('dblclick')
    await changedFile.trigger('dblclick')
    expect(AppService.OpenDiff.mock.calls).toEqual([
      ['/home/user/project', { kind: 'commit', sha: 'abc123', path: 'x.txt', oldPath: '' }],
      ['/home/user/project', { kind: 'working', staged: false, path: 'a.txt', oldPath: '' }],
    ])
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)
  })

  it('FR-CMP-02 a compared file opens its diff only in the diff window: a click shows none here, a double-click opens it', async () => {
    Facade.CompareCommits.mockResolvedValue([
      { path: 'a.txt', status: 'M' },
      { path: 'b.txt', status: 'A' },
    ])
    useLogStore().selection = [commitRow({ sha: 'sha-a' }), commitRow({ sha: 'sha-b' })]
    const wrapper = mountView({ path: '/home/user/project' })
    await flushPromises()
    const file = wrapper.findAll('.compare-view .files li')[1]!

    await file.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(DiffView).exists()).toBe(false)

    await file.trigger('dblclick')
    expect(AppService.OpenDiff).toHaveBeenCalledWith('/home/user/project', {
      kind: 'range',
      from: 'sha-a',
      to: 'sha-b',
      path: 'b.txt',
      oldPath: '',
    })
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
