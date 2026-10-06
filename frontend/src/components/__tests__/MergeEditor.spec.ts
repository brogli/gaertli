import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { SaveConflict: vi.fn() } }))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { flushPromises, mount, type DOMWrapper, type VueWrapper } from '@vue/test-utils'
import type { Extension } from '@codemirror/state'
import { EditorView, ViewPlugin } from '@codemirror/view'
import * as codeEditor from '@/lib/codeEditor'
import { editorViews } from '@/testing/editorView'
import MergeEditor from '../MergeEditor.vue'
import { mountInBody } from '@/testing/mountInBody'
import { findButton } from '@/testing/findButton'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { conflictFileFixture } from '@/testing/fixtures/conflicts'
import {
  Operation,
  type ConflictFile,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

beforeEach(() => useMergeDrafts().clearAll())

async function mountEditor(file: ConflictFile = conflictFileFixture()) {
  const { wrapper } = await mountInBody(MergeEditor, { props: { wt: '/repo', file } })
  return wrapper
}

// shown keeps the elements outside the hidden base panel.
function shown<T extends Element>(elements: DOMWrapper<T>[]): DOMWrapper<T>[] {
  return elements.filter((el) => !el.element.closest('[inert]'))
}

// shownViews are the editors outside the hidden base panel, in pane order.
function shownViews(wrapper: VueWrapper): EditorView[] {
  return editorViews(wrapper).filter((view) => !view.dom.closest('[inert]'))
}

function editorTexts(wrapper: VueWrapper): string[] {
  return shownViews(wrapper).map((view) => view.state.doc.toString())
}

function headers(wrapper: VueWrapper): string[] {
  return shown(wrapper.findAll('[data-test^="pane-header-"]')).map((h) => h.text())
}

describe('MergeEditor layout', () => {
  it('FR-CONF-05 shows the current side, the result and the incoming side', async () => {
    const file = conflictFileFixture()
    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)).toEqual([file.current.content, file.result, file.incoming.content])
    expect(headers(wrapper)).toEqual(['main · current branch', 'Result', 'side · merged branch'])
    expect(wrapper.get('[data-test="conflict-position"]').text()).toBe('Conflict 1 of 2 · 2 left')
  })

  it.each([
    [
      Operation.RebaseOperation,
      '',
      'main · rebased onto',
      'abcdef0 on side · commit being replayed',
    ],
    [Operation.CherryPickOperation, 'side', 'main · current branch', 'side · picked commit'],
    [
      Operation.RevertOperation,
      'side',
      'main · current branch',
      'abcdef0 on side · state before the reverted commit',
    ],
  ])('FR-CONF-05 names the panes of a %s', async (operation, incomingRef, current, incoming) => {
    const base = conflictFileFixture()
    const file = { ...base, operation, incoming: { ...base.incoming, ref: incomingRef } }
    const wrapper = await mountEditor(file)

    const [currentHeader, , incomingHeader] = headers(wrapper)
    expect([currentHeader, incomingHeader]).toEqual([current, incoming])
  })

  it('FR-CONF-05 shows the base pane on demand', async () => {
    const file = conflictFileFixture()
    const wrapper = await mountEditor(file)
    expect(editorTexts(wrapper)).toHaveLength(3)

    await findButton(wrapper, 'Show base')?.trigger('click')

    expect(editorTexts(wrapper)).toEqual([
      file.base.content,
      file.current.content,
      file.result,
      file.incoming.content,
    ])
    expect(headers(wrapper)[0]).toBe('Base')
  })

  it('FR-CONF-05 showing the base keeps every editor and its scroll position', async () => {
    const wrapper = await mountEditor()
    const hosts = () =>
      wrapper.findAll<HTMLElement>('.cm-editor').map((el) => el.element.parentElement)
    const before = hosts()
    const result = wrapper.findAll<HTMLElement>('.cm-scroller')[2]!.element
    result.scrollTop = 40

    await findButton(wrapper, 'Show base')?.trigger('click')
    await findButton(wrapper, 'Hide base')?.trigger('click')

    expect(hosts().every((host, i) => host === before[i])).toBe(true)
    expect(result.isConnected).toBe(true)
    expect(result.scrollTop).toBe(40)
  })

  it('FR-CONF-05 the hidden base pane has no divider to drag', async () => {
    const wrapper = await mountEditor()
    const baseGutter = () => wrapper.find('.p-splitter-vertical > .p-splitter-gutter')

    expect(baseGutter().classes()).toContain('hidden')

    await findButton(wrapper, 'Show base')?.trigger('click')

    expect(baseGutter().classes()).not.toContain('hidden')
  })

  it('FR-CONF-05 keeps the result when the base pane is shown', async () => {
    const wrapper = await mountEditor()
    const result = shownViews(wrapper)[1]!
    result.dispatch({ changes: { from: 0, insert: 'typed ' } })

    await findButton(wrapper, 'Show base')?.trigger('click')

    expect(editorTexts(wrapper)[2]).toMatch(/^typed a\n/)
  })

  it('FR-CONF-05 offers no base toggle when the file has no base', async () => {
    const file = { ...conflictFileFixture(), base: { present: false } }
    const wrapper = await mountEditor(file)

    expect(findButton(wrapper, 'Show base')).toBeUndefined()
  })
})

const CURRENT = 0
const RESULT = 1
const INCOMING = 2

function viewAt(wrapper: VueWrapper, pane: 0 | 1 | 2): EditorView {
  return shownViews(wrapper)[pane]!
}

function position(wrapper: VueWrapper): string {
  return wrapper.get('[data-test="conflict-position"]').text()
}

async function click(wrapper: VueWrapper, label: string): Promise<void> {
  await findButton(wrapper, label)?.trigger('click')
}

function press(view: EditorView, init: KeyboardEventInit): void {
  view.contentDOM.dispatchEvent(
    new KeyboardEvent('keydown', { ...init, bubbles: true, cancelable: true }),
  )
}

// threeRegionFile has one conflict region on each of the lines 2, 4 and 6 of
// both sides.
function threeRegionFile(): ConflictFile {
  const block = (n: number) => ['<<<<<<< HEAD', `c${n}`, '=======', `i${n}`, '>>>>>>> side']
  const result = ['a', ...block(1), 'b', ...block(2), 'c', ...block(3), 'z'].join('\n')
  const region = (n: number, resultStart: number) => ({
    result: { start: resultStart, lines: 5 },
    base: { start: 2 * n, lines: 1 },
    current: { start: 2 * n, lines: 1 },
    incoming: { start: 2 * n, lines: 1 },
  })
  const side = (prefix: string) => ['a', `${prefix}1`, 'b', `${prefix}2`, 'c', `${prefix}3`, 'z']
  const base = conflictFileFixture()
  return {
    ...base,
    result,
    regions: [region(1, 2), region(2, 8), region(3, 14)],
    current: { ...base.current, content: side('c').join('\n') },
    incoming: { ...base.incoming, content: side('i').join('\n') },
  }
}

// selectedLineCount is how many lines of the pane carry the selected mark.
function selectedLineCount(view: EditorView): number {
  return view.dom.querySelectorAll('.cm-conflict-selected').length
}

describe('MergeEditor actions and navigation', () => {
  it('FR-CONF-05 next moves to the next conflict, and the takes resolve both', async () => {
    const wrapper = await mountEditor()

    await click(wrapper, 'Next conflict · Alt+Down')
    expect(position(wrapper)).toBe('Conflict 2 of 2 · 2 left')
    const result = viewAt(wrapper, RESULT)
    expect(result.state.doc.lineAt(result.state.selection.main.head).number).toBe(8)

    await click(wrapper, 'Take current')
    expect(result.state.doc.toString()).toBe(
      'a\n<<<<<<< HEAD\nc1\n=======\ni1\n>>>>>>> side\nb\nc2\nz',
    )

    await click(wrapper, 'Previous conflict · Alt+Up')
    expect(position(wrapper)).toBe('Conflict 1 of 2 · 1 left')
    await click(wrapper, 'Both, incoming first')
    expect(result.state.doc.toString()).toBe('a\ni1\nc1\nb\nc2\nz')
    expect(position(wrapper)).toBe('No conflicts left')
  })

  it.each([
    ['Take incoming', 'a\ni1\nb\n<<<<<<< HEAD\nc2\n=======\ni2\n>>>>>>> side\nz'],
    ['Both, current first', 'a\nc1\ni1\nb\n<<<<<<< HEAD\nc2\n=======\ni2\n>>>>>>> side\nz'],
  ])('FR-CONF-05 "%s" acts on the selected conflict', async (label, text) => {
    const wrapper = await mountEditor()

    await click(wrapper, label)

    expect(viewAt(wrapper, RESULT).state.doc.toString()).toBe(text)
  })

  it('FR-CONF-05 disables the takes when no unresolved conflict is selected', async () => {
    const wrapper = await mountEditor()
    await click(wrapper, 'Take current')
    await click(wrapper, 'Next conflict · Alt+Down')
    await click(wrapper, 'Take current')

    for (const label of ['Take current', 'Take incoming', 'Both, current first']) {
      expect(findButton(wrapper, label)?.attributes('disabled')).toBeDefined()
    }
  })

  it('FR-CONF-05 clicking a conflict in the incoming pane selects it everywhere', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const incoming = viewAt(wrapper, INCOMING)

    incoming.dispatch({
      selection: { anchor: incoming.state.doc.line(6).from },
      userEvent: 'select.pointer',
    })
    await flushPromises()

    expect(position(wrapper)).toBe('Conflict 3 of 3 · 3 left')
    for (const pane of [CURRENT, RESULT, INCOMING] as const) {
      expect(selectedLineCount(viewAt(wrapper, pane))).toBeGreaterThan(0)
    }
    expect(selectedLineCount(viewAt(wrapper, RESULT))).toBe(5)

    press(incoming, { key: 'ArrowUp', altKey: true })
    await flushPromises()
    expect(position(wrapper)).toBe('Conflict 2 of 3 · 3 left')
  })

  it('FR-CONF-05 clicking a side conflict the restored result lacks leaves the selection alone and logs no error', async () => {
    const file = threeRegionFile()
    useMergeDrafts().set('/repo', 'a.txt', {
      text: file.result!,
      fingerprint: 'fp',
      regions: [{ start: 2, lines: 5 }],
    })
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountEditor(file)
    const incoming = viewAt(wrapper, INCOMING)

    incoming.dispatch({
      selection: { anchor: incoming.state.doc.line(6).from },
      userEvent: 'select.pointer',
    })
    await flushPromises()

    expect(logged).not.toHaveBeenCalled()
    expect(position(wrapper)).toBe('Conflict 1 of 1 · 1 left')
    logged.mockRestore()
  })

  it('FR-CONF-05 the takes stay disabled while the selected conflict holds no complete block', async () => {
    const wrapper = await mountEditor()
    const result = viewAt(wrapper, RESULT)
    const doc = result.state.doc
    result.dispatch({ changes: { from: doc.line(3).from, to: doc.line(9).to } })
    await flushPromises()

    expect(findButton(wrapper, 'Take current')?.attributes('disabled')).toBeDefined()
  })

  it('FR-CONF-05 the result cursor entering a conflict selects it', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const result = viewAt(wrapper, RESULT)

    result.dispatch({ selection: { anchor: result.state.doc.line(15).from } })
    await flushPromises()

    expect(position(wrapper)).toBe('Conflict 3 of 3 · 3 left')
  })

  it('FR-CONF-05 Alt+Down in the result moves to the next conflict, not the line', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const result = viewAt(wrapper, RESULT)

    press(result, { key: 'ArrowDown', altKey: true })
    await flushPromises()

    expect(position(wrapper)).toBe('Conflict 2 of 3 · 3 left')
    expect(result.state.doc.lineAt(result.state.selection.main.head).number).toBe(8)
    expect(result.state.doc.lines).toBe(19)
  })
})

const SAVE_AND_RESOLVE = 'Save and mark resolved'

describe('MergeEditor saving', () => {
  beforeEach(() => {
    Facade.SaveConflict.mockReset().mockResolvedValue('fp2')
  })

  it('FR-CONF-06 saves the result; marking resolved waits for the last conflict', async () => {
    const file = conflictFileFixture()
    const wrapper = await mountEditor(file)

    const resolve = findButton(wrapper, SAVE_AND_RESOLVE)!
    expect(resolve.attributes('disabled')).toBeDefined()
    expect(resolve.attributes('title')).toBe('Resolve every conflict first')
    expect(findButton(wrapper, 'Save')!.attributes('title')).toBe('Save · Ctrl+S')
    await click(wrapper, 'Save')

    expect(Facade.SaveConflict).toHaveBeenCalledWith('/repo', 'a.txt', file.result, 'fp', false)
  })

  it('FR-CONF-06 marks resolved once no marker line is left; Ctrl+S saves without marking', async () => {
    const wrapper = await mountEditor()
    await click(wrapper, 'Take current')
    await click(wrapper, 'Next conflict · Alt+Down')
    await click(wrapper, 'Both, incoming first')

    await click(wrapper, SAVE_AND_RESOLVE)
    expect(Facade.SaveConflict).toHaveBeenLastCalledWith(
      '/repo',
      'a.txt',
      'a\nc1\nb\ni2\nc2\nz',
      'fp',
      true,
    )

    press(viewAt(wrapper, RESULT), { key: 's', ctrlKey: true })
    await flushPromises()
    expect(Facade.SaveConflict).toHaveBeenLastCalledWith(
      '/repo',
      'a.txt',
      'a\nc1\nb\ni2\nc2\nz',
      'fp2',
      false,
    )
  })

  it('FR-CONF-06 a file that changed since it was opened is offered a reload', async () => {
    Facade.SaveConflict.mockRejectedValue(
      Object.assign(new Error('rejected'), {
        cause: {
          message: 'changed',
          key: 'fileChangedSinceOpened',
          args: { path: 'a.txt' },
        },
      }),
    )
    const wrapper = await mountEditor()

    await click(wrapper, 'Save')
    expect(wrapper.text()).toContain('a.txt changed since it was opened.')
    await click(wrapper, 'Reload')

    expect(wrapper.emitted('reload')).toHaveLength(1)
  })

  it('FR-CONF-06 any other save error shows as an error', async () => {
    Facade.SaveConflict.mockRejectedValue(new Error('disk full'))
    const wrapper = await mountEditor()

    await click(wrapper, 'Save')

    expect(wrapper.find('.error-banner').text()).toContain('disk full')
    expect(findButton(wrapper, 'Reload')).toBeUndefined()
  })
})

describe('MergeEditor toolbar and lifecycle', () => {
  it('FR-CONF-05 names the conflicts left next to the position', async () => {
    const wrapper = await mountEditor()
    await click(wrapper, 'Take current')

    expect(position(wrapper)).toBe('Conflict 1 of 2 · 1 left')
  })

  it('FR-CONF-06 a stray marker line explains the disabled Save and mark resolved', async () => {
    const wrapper = await mountEditor()
    await click(wrapper, 'Take current')
    await click(wrapper, 'Next conflict · Alt+Down')
    await click(wrapper, 'Take current')
    const result = viewAt(wrapper, RESULT)
    result.dispatch({ changes: { from: 0, insert: '>>>>>>> stray\n' } })
    await flushPromises()

    expect(position(wrapper)).toBe('No conflicts left')
    const resolve = findButton(wrapper, SAVE_AND_RESOLVE)!
    expect(resolve.attributes('disabled')).toBeDefined()
    expect(resolve.attributes('title')).toBe(
      'A line still starts with a conflict marker. Remove it, or mark the file resolved from the conflict list.',
    )
  })

  it('FR-CONF-06 creates no editor when it unmounts before the editor extensions load', async () => {
    let created = 0
    let destroyed = 0
    const counting = ViewPlugin.define(() => {
      created++
      return { destroy: () => destroyed++ }
    })
    let finish: (extensions: Extension[]) => void = () => {}
    vi.spyOn(codeEditor, 'codeEditorExtensions').mockReturnValueOnce(
      new Promise((resolve) => (finish = resolve)),
    )
    const wrapper = mount(MergeEditor, { props: { wt: '/repo', file: conflictFileFixture() } })

    wrapper.unmount()
    finish([counting])
    await flushPromises()

    expect(created).toBe(0)
    expect(destroyed).toBe(0)
  })
})

describe('MergeEditor saving while typing', () => {
  it('FR-CONF-06 typing during a save keeps the draft with the saved fingerprint', async () => {
    let finish: (fingerprint: string) => void = () => {}
    Facade.SaveConflict.mockReset().mockReturnValue(
      new Promise<string>((resolve) => (finish = resolve)),
    )
    const wrapper = await mountEditor()
    const result = viewAt(wrapper, RESULT)
    result.dispatch({ changes: { from: 0, insert: 'one ' } })
    await click(wrapper, 'Save')

    result.dispatch({ changes: { from: 0, insert: 'two ' } })
    finish('fpSaved')
    await flushPromises()

    const draft = useMergeDrafts().get('/repo', 'a.txt')
    expect(draft?.text).toMatch(/^two one a\n/)
    expect(draft?.fingerprint).toBe('fpSaved')
    expect(wrapper.emitted('saved')).toEqual([['fpSaved']])
  })

  it('FR-CONF-06 a save tells the new fingerprint', async () => {
    Facade.SaveConflict.mockReset().mockResolvedValue('fpSaved')
    const wrapper = await mountEditor()

    await click(wrapper, 'Save')

    expect(wrapper.emitted('saved')).toEqual([['fpSaved']])
  })
})

describe('MergeEditor unsaved results', () => {
  beforeEach(() => {
    Facade.SaveConflict.mockReset().mockResolvedValue('fp2')
  })

  async function editResult(wrapper: VueWrapper): Promise<void> {
    viewAt(wrapper, RESULT).dispatch({ changes: { from: 0, insert: 'typed ' } })
    await flushPromises()
  }

  it('FR-CONF-06 a result edited and left unsaved comes back when the file is shown again', async () => {
    const file = conflictFileFixture()
    const first = await mountEditor(file)
    await editResult(first)
    first.unmount()

    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)[RESULT]).toMatch(/^typed a\n/)
    expect(position(wrapper)).toBe('Conflict 1 of 2 · 2 left')
  })

  it('FR-CONF-06 a saved result leaves nothing to restore', async () => {
    const file = conflictFileFixture()
    const first = await mountEditor(file)
    await editResult(first)
    await click(first, 'Save')
    first.unmount()

    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)[RESULT]).toBe(file.result)
    expect(useMergeDrafts().hasUnsaved.value).toBe(false)
  })

  it('FR-CONF-06 unsaved edits stay when the file changed on disk, which Reload discards', async () => {
    const wrapper = await mountEditor()
    await editResult(wrapper)

    await wrapper.setProps({ file: conflictFileFixture({ fingerprint: 'fp2' }) })

    expect(wrapper.text()).toContain('a.txt changed on disk.')
    expect(editorTexts(wrapper)[RESULT]).toMatch(/^typed /)
    await click(wrapper, 'Reload')
    expect(wrapper.emitted('reload')).toHaveLength(1)
  })

  it('FR-CONF-06 the same file read again changes nothing', async () => {
    const wrapper = await mountEditor()
    await editResult(wrapper)

    await wrapper.setProps({ file: conflictFileFixture() })

    expect(wrapper.text()).not.toContain('changed on disk')
  })
})
