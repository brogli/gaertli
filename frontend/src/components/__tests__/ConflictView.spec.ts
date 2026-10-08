import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { undo } from '@codemirror/commands'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '@/testing/mountInBody'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    Conflict: vi.fn(),
    MergeDraft: vi.fn(),
    SetMergeDraft: vi.fn(),
    SaveConflict: vi.fn(),
    ResolveConflict: vi.fn(),
    StageFile: vi.fn(),
    OpenFile: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))
vi.mock('openvue/useconfirm', () => import('@/testing/confirm'))
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import ConflictView from '../ConflictView.vue'
import { confirmOptions, confirmRequire } from '@/testing/confirm'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { emit } from '@/testing/runtime'
import { ChangeScope } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { findButton } from '@/testing/findButton'
import { editorViews } from '@/testing/editorView'
import { conflictFileFixture } from '@/testing/fixtures/conflicts'
import {
  ConflictChoice,
  ConflictKind,
  Operation,
  type ConflictFile,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

async function mountView(file: Partial<ConflictFile>) {
  Facade.Conflict.mockResolvedValue(conflictFileFixture(file))
  const { wrapper } = await mountInBody(ConflictView, { props: { wt: '/repo', path: 'a.txt' } })
  return wrapper
}

// resolveAll takes the current side in both conflicts of the fixture.
async function resolveAll(wrapper: VueWrapper): Promise<void> {
  for (const label of ['Take left', 'Next change · Alt+Down', 'Take left']) {
    await findButton(wrapper, label)!.trigger('click')
  }
}

describe('ConflictView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const fn of Object.values(Facade)) fn.mockReset().mockResolvedValue(undefined)
    confirmRequire.mockReset()
    useMergeDrafts().keepOnly('/repo', [])
    document.body.innerHTML = ''
  })

  it('FR-CONF-07 a file the current side deleted offers the incoming version and deletion', async () => {
    const wrapper = await mountView({
      kind: ConflictKind.ConflictDeleted,
      current: { ...conflictFileFixture().current, present: false },
    })

    expect(wrapper.text()).toContain('main deleted the file; side changed it.')
    expect(findButton(wrapper, `Keep the version of main`)).toBeUndefined()
    await findButton(wrapper, `Take the version of side`)!.trigger('click')
    expect(Facade.ResolveConflict).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      ConflictChoice.ChooseIncoming,
    )
    expect(findButton(wrapper, 'Delete file…')).toBeDefined()
  })

  it('FR-CONF-07 a file the incoming side deleted offers the current version', async () => {
    const wrapper = await mountView({
      kind: ConflictKind.ConflictDeleted,
      incoming: { ...conflictFileFixture().incoming, present: false },
    })

    expect(wrapper.text()).toContain('side deleted the file; main changed it.')
    expect(findButton(wrapper, `Take the version of side`)).toBeUndefined()
    await findButton(wrapper, `Keep the version of main`)!.trigger('click')
    expect(Facade.ResolveConflict).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      ConflictChoice.ChooseCurrent,
    )
  })

  const { current, incoming } = conflictFileFixture()
  it.each([
    [
      'FR-CONF-07 a binary conflict',
      { kind: ConflictKind.ConflictBinary },
      'Both sides changed this binary file.',
      ['Keep the version of main', 'Take the version of side'],
    ],
    [
      'FR-CONF-07 a symlink conflict',
      { kind: ConflictKind.ConflictSymlink },
      'Both sides changed this symbolic link.',
      ['Keep the version of main', 'Take the version of side'],
    ],
    [
      'FR-CONF-05 FR-CONF-07 a revert, naming its incoming side as the state before the commit,',
      { kind: ConflictKind.ConflictBinary, operation: Operation.RevertOperation },
      'Both sides changed this binary file.',
      ['Keep the version of main', 'Take the version before abcdef0 on side'],
    ],
    [
      'FR-CONF-05 FR-CONF-07 a conflict between sides without a commit, naming them the current and incoming side,',
      {
        kind: ConflictKind.ConflictBinary,
        operation: Operation.$zero,
        current: { ...current, sha: '', ref: '' },
        incoming: { ...incoming, sha: '', ref: '' },
      },
      'Both sides changed this binary file.',
      ['Keep the version of Current side', 'Take the version of Incoming side'],
    ],
  ])('%s offers both versions; Delete file confirms first', async (_, file, sentence, labels) => {
    const wrapper = await mountView(file)

    expect(wrapper.text()).toContain(sentence)
    await findButton(wrapper, labels[0]!)!.trigger('click')
    await findButton(wrapper, labels[1]!)!.trigger('click')
    expect(Facade.ResolveConflict.mock.calls.map((call) => call[2])).toEqual([
      ConflictChoice.ChooseCurrent,
      ConflictChoice.ChooseIncoming,
    ])

    Facade.ResolveConflict.mockClear()
    await findButton(wrapper, 'Delete file…')!.trigger('click')
    expect(confirmOptions().message).toBe(
      'Delete a.txt? The file is removed from the working tree and the index.',
    )
    expect(Facade.ResolveConflict).not.toHaveBeenCalled()
    confirmOptions().accept()
    expect(Facade.ResolveConflict).toHaveBeenCalledWith(
      '/repo',
      'a.txt',
      ConflictChoice.ChooseDelete,
    )
  })

  it.each([
    [ConflictKind.ConflictNotUTF8, 'This file is not UTF-8 text.'],
    [ConflictKind.ConflictRename, 'The two sides renamed this file differently.'],
    [ConflictKind.ConflictSubmodule, 'This is a submodule.'],
  ])(
    'FR-CONF-02 a %s conflict says why, offers the editor and Mark resolved',
    async (kind, reason) => {
      const wrapper = await mountView({ kind })

      expect(wrapper.text()).toContain(reason)
      await findButton(wrapper, 'Open in editor')!.trigger('click')
      expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
      await findButton(wrapper, 'Mark resolved')!.trigger('click')
      expect(Facade.StageFile).toHaveBeenCalledWith('/repo', 'a.txt')
    },
  )

  it.each([ConflictKind.ConflictBinary, ConflictKind.ConflictText])(
    'FR-CONF-02 previous and next file ask the parent on a %s conflict',
    async (kind) => {
      const wrapper = await mountView({ kind })

      await findButton(wrapper, 'Previous file')!.trigger('click')
      await findButton(wrapper, 'Next file')!.trigger('click')
      await flushPromises()

      expect(wrapper.emitted('step-file')).toEqual([[-1], [1]])
    },
  )

  it('FR-CONF-06 reloading after a refused save shows the file as it is on disk, without the edits', async () => {
    Facade.SaveConflict.mockRejectedValue(
      Object.assign(new Error('rejected'), {
        cause: { message: 'changed', key: 'fileChangedSinceOpened', args: { path: 'a.txt' } },
      }),
    )
    const wrapper = await mountView({})
    await resolveAll(wrapper)
    const resultView = () => editorViews(wrapper)[1]!
    resultView().dispatch({ changes: { from: 0, insert: 'typed ' } })
    await findButton(wrapper, 'Save and mark resolved')!.trigger('click')
    Facade.Conflict.mockResolvedValue(
      conflictFileFixture({ result: 'on disk', regions: [], fingerprint: 'fp2' }),
    )

    expect(wrapper.text()).toContain('a.txt changed since it was opened.')
    await findButton(wrapper, 'Reload')!.trigger('click')
    await flushPromises()

    expect(resultView().state.doc.toString()).toBe('on disk')
    expect(findButton(wrapper, 'Reload')).toBeUndefined()
  })

  describe('reading the conflict again', () => {
    const resultView = (wrapper: VueWrapper) => editorViews(wrapper)[1]!

    async function reread(file: Partial<ConflictFile>): Promise<void> {
      Facade.Conflict.mockResolvedValue(conflictFileFixture(file))
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
    }

    it('FR-CONF-06 the same fingerprint keeps the editor as it is', async () => {
      const wrapper = await mountView({})
      const before = resultView(wrapper)
      before.dispatch({ changes: { from: 0, insert: 'typed ' } })

      await reread({})

      expect(resultView(wrapper)).toBe(before)
      expect(before.state.doc.toString()).toMatch(/^typed /)
      expect(wrapper.text()).not.toContain('changed on disk')
    })

    it('FR-CONF-06 a save keeps the editor, its edits and undo through the reload it causes', async () => {
      Facade.SaveConflict.mockResolvedValue('fpSaved')
      const wrapper = await mountView({})
      await resolveAll(wrapper)
      const before = resultView(wrapper)
      before.dispatch({ changes: { from: 0, insert: 'typed ' }, userEvent: 'input.type' })
      await findButton(wrapper, 'Save and mark resolved')!.trigger('click')
      await flushPromises()

      await reread({ fingerprint: 'fpSaved', result: 'typed a' })

      expect(resultView(wrapper)).toBe(before)
      expect(before.state.doc.toString()).toMatch(/^typed a\n/)
      expect(undo(before)).toBe(true)
      expect(before.state.doc.toString()).toMatch(/^a\n/)
    })

    it('FR-CONF-06 a read that started before a save does not undo the save', async () => {
      Facade.SaveConflict.mockResolvedValue('fpSaved')
      const wrapper = await mountView({})
      await resolveAll(wrapper)
      const before = resultView(wrapper)
      before.dispatch({ changes: { from: 0, insert: 'typed ' }, userEvent: 'input.type' })
      let answerStaleRead: (file: ConflictFile) => void = () => undefined
      Facade.Conflict.mockReturnValueOnce(
        new Promise<ConflictFile>((resolve) => (answerStaleRead = resolve)),
      )
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await findButton(wrapper, 'Save and mark resolved')!.trigger('click')
      await flushPromises()
      answerStaleRead(conflictFileFixture({}))
      await flushPromises()

      await reread({ fingerprint: 'fpSaved', result: 'typed a' })

      expect(resultView(wrapper)).toBe(before)
    })

    it('FR-CONF-06 a read that fails after a newer read started shows no error', async () => {
      const wrapper = await mountView({})
      let failStaleRead: (e: Error) => void = () => undefined
      Facade.Conflict.mockReturnValueOnce(
        new Promise<ConflictFile>((_, reject) => (failStaleRead = reject)),
      )
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
      Facade.Conflict.mockResolvedValue(conflictFileFixture({}))
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()
      failStaleRead(Object.assign(new Error('boom'), { cause: { message: 'boom' } }))
      await flushPromises()

      expect(wrapper.find('.error-banner').exists()).toBe(false)
    })

    it('FR-CONF-06 a read of a file that is no longer conflicted shows no error', async () => {
      const wrapper = await mountView({})
      Facade.Conflict.mockRejectedValue(
        Object.assign(new Error('rejected'), {
          cause: { message: 'not conflicted', key: 'notConflicted', args: { path: 'a.txt' } },
        }),
      )
      emit('repo:changed', { data: { scope: ChangeScope.IndexChanged } })
      await flushPromises()

      expect(wrapper.find('.error-banner').exists()).toBe(false)
    })

    it('FR-CONF-06 a save that ends after a switch to another file leaves that file shown', async () => {
      let finishSave: (fingerprint: string) => void = () => undefined
      Facade.SaveConflict.mockReturnValueOnce(
        new Promise<string>((resolve) => (finishSave = resolve)),
      )
      const wrapper = await mountView({})
      await resolveAll(wrapper)
      resultView(wrapper).dispatch({ changes: { from: 0, insert: 'typed ' } })
      await findButton(wrapper, 'Save and mark resolved')!.trigger('click')
      let answerRead: (file: ConflictFile) => void = () => undefined
      Facade.Conflict.mockReturnValueOnce(
        new Promise<ConflictFile>((resolve) => (answerRead = resolve)),
      )
      await wrapper.setProps({ path: 'b.txt' })

      finishSave('fpA2')
      await flushPromises()
      answerRead(conflictFileFixture({ path: 'b.txt', result: 'other', regions: [] }))
      await flushPromises()

      expect(resultView(wrapper).state.doc.toString()).toBe('other')
    })

    it('FR-CONF-06 a changed file shows as it is now, but keeps unsaved edits and says so; Reload drops them', async () => {
      const wrapper = await mountView({})
      await reread({ result: 'on disk', regions: [], fingerprint: 'fp2' })
      expect(resultView(wrapper).state.doc.toString()).toBe('on disk')

      resultView(wrapper).dispatch({ changes: { from: 0, insert: 'typed ' } })
      await reread({ result: 'on disk again', regions: [], fingerprint: 'fp3' })
      expect(resultView(wrapper).state.doc.toString()).toMatch(/^typed /)
      expect(wrapper.text()).toContain('a.txt changed on disk.')

      await findButton(wrapper, 'Reload')!.trigger('click')
      await flushPromises()
      expect(resultView(wrapper).state.doc.toString()).toBe('on disk again')
      expect(useMergeDrafts().get('/repo', 'a.txt', '')).toBeUndefined()
      expect(Facade.SetMergeDraft).toHaveBeenLastCalledWith('/repo', 'a.txt', null)
    })

    it.each([
      ['a changed file', { result: 'on disk', regions: [], fingerprint: 'fp2' }],
      ['the same file', {}],
    ])(
      'FR-CONF-06 %s read as a new conflict in the path shows afresh, without the edits made for the earlier one',
      async (_, file) => {
        const wrapper = await mountView({ conflictId: 'c1' })
        resultView(wrapper).dispatch({ changes: { from: 0, insert: 'typed ' } })
        expect(Facade.SetMergeDraft.mock.lastCall![2].conflictId).toBe('c1')

        await reread({ ...file, conflictId: 'c2' })

        expect(resultView(wrapper).state.doc.toString()).not.toMatch(/^typed /)
        expect(wrapper.text()).not.toContain('changed on disk')
        expect(useMergeDrafts().get('/repo', 'a.txt', 'c2')).toBeUndefined()
      },
    )

    it('FR-CONF-06 Reload reads the draft only after the core dropped it', async () => {
      const wrapper = await mountView({})
      resultView(wrapper).dispatch({ changes: { from: 0, insert: 'typed ' } })
      const [, , stored] = Facade.SetMergeDraft.mock.lastCall!
      await reread({ result: 'on disk', regions: [], fingerprint: 'fp2' })
      let cored: unknown = stored
      let finishWrite: () => void = () => undefined
      Facade.SetMergeDraft.mockImplementation(
        (_wt: string, _path: string, draft: unknown) =>
          new Promise<void>((resolve) => {
            finishWrite = () => {
              cored = draft
              resolve()
            }
          }),
      )
      Facade.MergeDraft.mockImplementation(() => Promise.resolve(cored))

      await findButton(wrapper, 'Reload')!.trigger('click')
      await flushPromises()
      finishWrite()
      await flushPromises()

      expect(resultView(wrapper).state.doc.toString()).toBe('on disk')
    })

    it('FR-CONF-06 a taken region and the edits survive a new page: the core returns them', async () => {
      const wrapper = await mountView({})
      await findButton(wrapper, 'Take left')!.trigger('click')
      resultView(wrapper).dispatch({ changes: { from: 0, insert: 'typed ' } })
      const text = resultView(wrapper).state.doc.toString()
      wrapper.unmount()
      const [, , stored] = Facade.SetMergeDraft.mock.lastCall!
      expect(stored.text).toBe(text)

      useMergeDrafts().keepOnly('/repo', [])
      Facade.MergeDraft.mockResolvedValue(stored)
      const reopened = await mountView({})

      expect(Facade.MergeDraft).toHaveBeenCalledWith('/repo', 'a.txt')
      expect(resultView(reopened).state.doc.toString()).toBe(text)
      expect(reopened.text()).toContain('1 left')
    })

    it('FR-CONF-06 edits reach the core while typing: the first at once, a burst after a pause', async () => {
      vi.useFakeTimers()
      try {
        const wrapper = await mountView({})
        resultView(wrapper).dispatch({ changes: { from: 0, insert: 'one ' } })
        expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(1)
        resultView(wrapper).dispatch({ changes: { from: 0, insert: 'two ' } })
        expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(1)

        await vi.advanceTimersByTimeAsync(1000)

        expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(2)
        expect(Facade.SetMergeDraft.mock.lastCall![2].text).toMatch(/^two one a\n/)
      } finally {
        vi.useRealTimers()
      }
    })

    it('FR-CONF-06 edits still waiting reach the core when the window closes', async () => {
      vi.useFakeTimers()
      try {
        const wrapper = await mountView({})
        resultView(wrapper).dispatch({ changes: { from: 0, insert: 'one ' } })
        resultView(wrapper).dispatch({ changes: { from: 0, insert: 'two ' } })

        window.dispatchEvent(new Event('pagehide'))

        expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(2)
        expect(Facade.SetMergeDraft.mock.lastCall![2].text).toMatch(/^two one a\n/)
      } finally {
        vi.useRealTimers()
      }
    })
  })
})
