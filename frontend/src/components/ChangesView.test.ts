import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'
import NoticeArea from './NoticeArea.vue'

const { Facade, AppService } = vi.hoisted(() => ({
  AppService: { OpenDiff: vi.fn() },
  Facade: {
    StageFile: vi.fn(),
    UnstageFile: vi.fn(),
    StageAll: vi.fn(),
    UnstageAll: vi.fn(),
    DiscardFile: vi.fn(),
    Status: vi.fn(),
    OpenFile: vi.fn(),
    Commit: vi.fn(),
    SetMessageDraft: vi.fn(),
  },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

vi.mock('openvue/useconfirm', () => import('../testing/confirm'))
import { confirmOptions, confirmRequire } from '../testing/confirm'

import ChangesView from './ChangesView.vue'
import CommitBox from './CommitBox.vue'
import { useChangesStore } from '../stores/changes'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { findButton } from '../testing/findButton'
import type { ChangedFiles } from '../../bindings/github.com/brogli/nebegit/internal/core'

const props = { wt: '/repo' }

// mountWith mounts ChangesView over a change list holding files.
function mountWith(files: Partial<ChangedFiles>) {
  useChangesStore().files = changedFilesFixture(files)
  return mount(ChangesView, { props })
}

describe('ChangesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const fn of Object.values(Facade)) fn.mockReset().mockResolvedValue(undefined)
    Facade.Status.mockResolvedValue(changedFilesFixture())
    confirmRequire.mockReset()
    AppService.OpenDiff.mockReset()
  })

  it('FR-CHG-11 CommitBox renders after the change list, outside its scrolling region', () => {
    const wrapper = mount(ChangesView, { props })

    expect(wrapper.findComponent(CommitBox).exists()).toBe(true)
    const html = wrapper.html()
    expect(html.indexOf('Staged')).toBeLessThan(html.indexOf('class="commit-box"'))
    expect(wrapper.get('.changes-body').find('.commit-box').exists()).toBe(false)
  })

  it("FR-CHG-12 the commit message field has 6 rows and fills the commit box's width", () => {
    const wrapper = mount(ChangesView, { props })

    const textarea = wrapper.get('textarea')
    expect(textarea.attributes('rows')).toBe('6')
    expect(textarea.classes()).toContain('p-textarea-fluid')
  })

  it("FR-CHG-08 switching worktree remounts CommitBox, so worktree A's typed text never carries into worktree B's draft", async () => {
    const wrapper = mount(ChangesView, { props })
    await wrapper.get('textarea').setValue('worktree A text')

    await wrapper.setProps({ wt: '/other' })

    expect(wrapper.get('textarea').element.value).toBe('')
  })

  it("FR-CHG-02 clicking an unstaged row's Stage button calls Facade.StageFile with its path", async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.StageFile).toHaveBeenCalledTimes(1)
    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it("FR-CHG-02 clicking a staged row's Unstage button calls Facade.UnstageFile with its path", async () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.UnstageFile).toHaveBeenCalledTimes(1)
    expect(Facade.UnstageFile).toHaveBeenCalledWith('/repo', 'a.txt')
  })

  it('FR-CHG-04 a click only marks a row; a double-click or Enter opens its side of the change in the diff window', async () => {
    const wrapper = mountWith({
      staged: [{ path: 'a.txt', status: 'M' }],
      unstaged: [{ path: 'a.txt', status: 'M' }],
    })
    const [staged, unstaged] = wrapper.findAll('li')

    await unstaged!.trigger('click')
    expect(unstaged!.classes()).toContain('selected')
    expect(staged!.classes()).not.toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    await unstaged!.trigger('dblclick')
    await staged!.trigger('keydown', { key: 'Enter' })

    expect(AppService.OpenDiff.mock.calls).toEqual([
      ['/repo', { kind: 'working', staged: false, path: 'a.txt', oldPath: '' }],
      ['/repo', { kind: 'working', staged: true, path: 'a.txt', oldPath: '' }],
    ])
  })

  it('FR-ED-02 clicking a row\'s "Open in editor" control calls Facade.OpenFile, not select', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
    expect(wrapper.get('li').classes()).not.toContain('selected')
  })

  it('FR-ED-06 OpenFile failing EDITOR_NOT_FOUND shows an error notice naming the command, no banner', async () => {
    Facade.OpenFile.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'EDITOR_NOT_FOUND', message: '"myed" is not on PATH' } }),
    )
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const { body: notice } = await mountInBody(NoticeArea)

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.text()).toContain('myed')
    expect(wrapper.find('.error-banner').exists()).toBe(false)
  })

  it("FR-UX-15 a second EDITOR_NOT_FOUND failure replaces the first one's notice", async () => {
    const notFound = (editor: string) =>
      new Error('boom', {
        cause: { code: 'EDITOR_NOT_FOUND', message: `"${editor}" is not on PATH` },
      })
    Facade.OpenFile.mockRejectedValueOnce(notFound('myed')).mockRejectedValueOnce(notFound('editor'))
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const { body: notice } = await mountInBody(NoticeArea)

    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(notice.findAll('.p-toast-message')).toHaveLength(1)
    expect(notice.text()).toContain('editor')
  })

  it('FR-CHG-02 "Stage all" calls Facade.StageAll once; the index event reloads the list', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    const stageAll = findButton(wrapper, 'Stage all')
    await stageAll?.trigger('click')
    await flushPromises()

    expect(Facade.StageAll).toHaveBeenCalledWith('/repo')
    expect(Facade.Status).not.toHaveBeenCalled()
  })

  it('FR-CONF-02 double-clicking a conflicted row calls Facade.OpenFile with its path', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    await wrapper.get('li').trigger('dblclick')

    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'c.txt', 0)
  })

  it('FR-CONF-03 clicking a conflicted row\'s "Mark resolved" button calls Facade.StageFile with its path', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    expect(wrapper.get('[data-test="action"]').attributes('aria-label')).toBe('Mark resolved')
    await wrapper.get('[data-test="action"]').trigger('click')

    expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'c.txt')
  })

  it('FR-CONF-02 a conflicted row is not selectable and opens no diff window', async () => {
    const wrapper = mountWith({ conflicted: [{ path: 'c.txt', status: 'UU' }] })
    await wrapper.get('li').trigger('click')
    await wrapper.get('li').trigger('dblclick')

    expect(wrapper.get('li').classes()).not.toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()
  })

  it('FR-CHG-10, FR-UX-03, QA-SAFE-01 an unstaged row\'s "Discard" confirms naming the path, then calls Facade.DiscardFile untracked=false', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    expect(confirmRequire).toHaveBeenCalledTimes(1)
    const options = confirmOptions()
    expect(options.message).toContain('a.txt')
    expect(Facade.DiscardFile).not.toHaveBeenCalled()

    options.accept()

    expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', 'a.txt', false)
  })

  it('FR-CHG-10 declining the confirm does not call Facade.DiscardFile', async () => {
    const wrapper = mountWith({ unstaged: [{ path: 'a.txt', status: 'M' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    expect(Facade.DiscardFile).not.toHaveBeenCalled()
  })

  it('FR-CHG-10 an untracked row\'s "Discard" confirm wording says the file will be deleted, then calls Facade.DiscardFile untracked=true', async () => {
    const wrapper = mountWith({ untracked: [{ path: 'new.txt', status: '?' }] })
    await findButton(wrapper, 'Discard')!.trigger('click')

    const options = confirmOptions()
    expect(options.message.toLowerCase()).toContain('delete')

    options.accept()

    expect(Facade.DiscardFile).toHaveBeenCalledWith('/repo', 'new.txt', true)
  })

  it('FR-UX-13 an untracked and a conflicted file show their status icons', () => {
    const wrapper = mountWith({
      untracked: [{ path: 'new.txt', status: '?' }],
      conflicted: [{ path: 'c.txt', status: 'UU' }],
    })

    const icons = wrapper.findAll('.status i').map((i) => i.classes())
    expect(icons).toContainEqual(['oi', 'oi-question-circle'])
    expect(icons).toContainEqual(['oi', 'oi-exclamation-triangle'])
  })

  it('FR-CHG-10 the Staged group offers no Discard button', () => {
    const wrapper = mountWith({ staged: [{ path: 'a.txt', status: 'M' }] })

    expect(wrapper.findAll('[data-test="second-action"]').length).toBe(0)
  })
})
