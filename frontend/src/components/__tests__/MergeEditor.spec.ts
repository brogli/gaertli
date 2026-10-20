import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: { SaveConflict: vi.fn(), SetMergeDraft: vi.fn(), MergeDraft: vi.fn() },
}))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { type Extension } from '@codemirror/state'
import { undo } from '@codemirror/commands'
import { EditorView, ViewPlugin } from '@codemirror/view'
import * as codeEditor from '@/lib/codeEditor'
import { editorViews } from '@/testing/editorView'
import MergeEditor from '../MergeEditor.vue'
import { mountInBody } from '@/testing/mountInBody'
import { findButton } from '@/testing/findButton'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { changesFileFixture, conflictFileFixture } from '@/testing/fixtures/conflicts'
import { resolveBothConflicts } from '@/testing/resolveConflicts'
import {
  RegionKind,
  type ConflictFile,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

beforeEach(() => useMergeDrafts().keepOnly('/repo', []))

async function mountEditor(file: ConflictFile = conflictFileFixture()) {
  const { wrapper } = await mountInBody(MergeEditor, { props: { wt: '/repo', file } })
  return wrapper
}

function editorTexts(wrapper: VueWrapper): string[] {
  return editorViews(wrapper).map((view) => view.state.doc.toString())
}

function headers(wrapper: VueWrapper): string[] {
  return wrapper.findAll('[data-test^="pane-header-"]').map((h) => h.text())
}

const CURRENT = 0
const RESULT = 1
const INCOMING = 2

function viewAt(wrapper: VueWrapper, pane: 0 | 1 | 2): EditorView {
  return editorViews(wrapper)[pane]!
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

// tintsOf is the tint class each line of the pane carries, if any.
function tintsOf(view: EditorView): (string | undefined)[] {
  const names = [
    'cm-change-added',
    'cm-change-changed',
    'cm-conflict-unresolved',
    'cm-conflict-resolved',
  ]
  return [...view.dom.querySelectorAll('.cm-line')].map((line) =>
    names.find((name) => line.classList.contains(name)),
  )
}

function markedLines(view: EditorView): number[] {
  return [...view.dom.querySelectorAll('.cm-line')].flatMap((line, i) =>
    line.classList.contains('cm-chunk-selected') ? [i + 1] : [],
  )
}

describe('MergeEditor layout', () => {
  it('FR-CONF-05 shows the current side, the result holding the base text in each conflict with no marker line, and the incoming side', async () => {
    const file = conflictFileFixture()
    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)).toEqual([
      file.current.content,
      'a\no1\nb\no2\nz',
      file.incoming.content,
    ])
    expect(headers(wrapper)).toEqual(['main · current branch', 'Result', 'side · merged branch'])
    expect(position(wrapper)).toBe('Change 1 of 2 · 2 left')
    const result = viewAt(wrapper, RESULT)
    expect(result.state.doc.toString()).not.toMatch(/^[<>]{7}/m)
    expect(result.dom.querySelectorAll('.cm-conflict-unresolved')).toHaveLength(2)
  })

  it('FR-CONF-05 the side panes tint conflicts red and show lines both sides changed alike as handled', async () => {
    const file = conflictFileFixture({
      currentChanges: [{ lines: { start: 1, lines: 1 }, added: true }],
      incomingChanges: [{ lines: { start: 5, lines: 1 }, added: false }],
    })
    const wrapper = await mountEditor(file)

    expect(tintsOf(viewAt(wrapper, CURRENT))).toEqual([
      'cm-conflict-resolved',
      'cm-conflict-unresolved',
      undefined,
      'cm-conflict-unresolved',
      undefined,
    ])
    expect(tintsOf(viewAt(wrapper, INCOMING))).toEqual([
      undefined,
      'cm-conflict-unresolved',
      undefined,
      'cm-conflict-unresolved',
      'cm-conflict-resolved',
    ])
  })

  it('FR-CONF-05 an unresolved conflict with no lines in the result shows a placeholder until it holds lines', async () => {
    const base = conflictFileFixture()
    const file = {
      ...base,
      base: { present: false },
      regions: base.regions!.map((region) => ({ ...region, base: { start: 0, lines: 0 } })),
    }
    const wrapper = await mountEditor(file)
    const result = viewAt(wrapper, RESULT)
    const placeholders = () => result.dom.querySelectorAll('.cm-conflict-placeholder')

    expect(placeholders()).toHaveLength(2)
    expect(placeholders()[0]!.textContent).toBe('Unresolved change')
    placeholders()[1]!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    await flushPromises()
    expect(position(wrapper)).toBe('Change 2 of 2 · 2 left')

    await click(wrapper, 'Take left')

    expect(placeholders()).toHaveLength(1)
    expect(result.state.doc.toString()).toBe('a\nb\nc2\nz')
  })

  it('FR-CONF-05 a region git trimmed to the differing lines holds the base lines of those only', async () => {
    const base = conflictFileFixture()
    const file: ConflictFile = {
      ...base,
      base: { present: true, content: 'a\nb\nc\nx' },
      current: { ...base.current, content: 'A\nB\nC\nx' },
      incoming: { ...base.incoming, content: 'A\nY\nC\nx' },
      result: 'A\n<<<<<<< HEAD\nB\n=======\nY\n>>>>>>> side\nC\nx',
      regions: [
        {
          kind: RegionKind.RegionConflict,
          result: { start: 2, lines: 5 },
          base: { start: 2, lines: 1 },
          current: { start: 2, lines: 1 },
          incoming: { start: 2, lines: 1 },
        },
      ],
      currentChanges: [],
      incomingChanges: [],
    }
    const wrapper = await mountEditor(file)
    const result = viewAt(wrapper, RESULT)

    expect(result.state.doc.toString()).toBe('A\nb\nC\nx')

    await click(wrapper, 'Take right')

    expect(result.state.doc.toString()).toBe('A\nY\nC\nx')
  })

  it('FR-CONF-10 pressing the placeholder of a region the incoming side deleted selects it', async () => {
    const base = conflictFileFixture()
    const file = {
      ...base,
      incoming: { ...base.incoming, content: 'a\ni1\nb\nz' },
      regions: [base.regions![0]!, { ...base.regions![1]!, incoming: { start: 4, lines: 0 } }],
    }
    const wrapper = await mountEditor(file)
    const placeholder = viewAt(wrapper, INCOMING).dom.querySelector('.cm-conflict-placeholder')!

    placeholder.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    await flushPromises()

    expect(position(wrapper)).toBe('Change 2 of 2 · 2 left')
  })
})

// threeRegionFile has one conflict region on each of the lines 2, 4 and 6 of
// both sides.
function threeRegionFile(): ConflictFile {
  const block = (n: number) => ['<<<<<<< HEAD', `c${n}`, '=======', `i${n}`, '>>>>>>> side']
  const result = ['a', ...block(1), 'b', ...block(2), 'c', ...block(3), 'z'].join('\n')
  const region = (n: number, resultStart: number) => ({
    kind: RegionKind.RegionConflict,
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
    base: { ...base.base, content: side('o').join('\n') },
    current: { ...base.current, content: side('c').join('\n') },
    incoming: { ...base.incoming, content: side('i').join('\n') },
  }
}

// selectedLineCount is how many lines of the pane carry the selected mark.
function selectedLineCount(view: EditorView): number {
  return view.dom.querySelectorAll('.cm-chunk-selected').length
}

describe('MergeEditor actions and navigation', () => {
  it('FR-CONF-10 next moves to the next conflict, and the takes resolve both', async () => {
    const wrapper = await mountEditor()

    await click(wrapper, 'Next change · Alt+Down')
    expect(position(wrapper)).toBe('Change 2 of 2 · 2 left')
    const result = viewAt(wrapper, RESULT)
    expect(result.state.doc.lineAt(result.state.selection.main.head).number).toBe(4)

    await click(wrapper, 'Take left')
    expect(result.state.doc.toString()).toBe('a\no1\nb\nc2\nz')

    await click(wrapper, 'Previous change · Alt+Up')
    expect(position(wrapper)).toBe('Change 1 of 2 · 1 left')
    await click(wrapper, 'Both, right first')
    expect(result.state.doc.toString()).toBe('a\ni1\nc1\nb\nc2\nz')
    expect(position(wrapper)).toBe('No changes left')
  })

  it.each([
    ['Take right', 'a\ni1\nb\no2\nz'],
    ['Both, left first', 'a\nc1\ni1\nb\no2\nz'],
  ])('FR-CONF-10 "%s" acts on the selected conflict', async (label, text) => {
    const wrapper = await mountEditor()

    await click(wrapper, label)

    expect(viewAt(wrapper, RESULT).state.doc.toString()).toBe(text)
  })

  it('FR-CONF-10 take incoming resolves the region in every pane', async () => {
    const wrapper = await mountEditor()

    await click(wrapper, 'Take right')

    for (const pane of [CURRENT, RESULT, INCOMING] as const) {
      const resolved = viewAt(wrapper, pane).dom.querySelectorAll('.cm-conflict-resolved')
      expect(resolved).toHaveLength(1)
    }
    expect(position(wrapper)).toBe('Change 1 of 2 · 1 left')
  })

  it('FR-CONF-10 a hand edit leaves the region unresolved until Mark done, which undo reverts', async () => {
    const wrapper = await mountEditor()
    const result = viewAt(wrapper, RESULT)
    result.dispatch({ changes: { from: result.state.doc.line(2).to, insert: ' mine' } })
    await flushPromises()
    expect(position(wrapper)).toBe('Change 1 of 2 · 2 left')

    await click(wrapper, 'Mark done')
    expect(position(wrapper)).toBe('Change 1 of 2 · 1 left')
    expect(result.state.doc.line(2).text).toBe('o1 mine')
    expect(findButton(wrapper, 'Mark done')?.attributes('disabled')).toBeDefined()

    undo(result)
    await flushPromises()
    expect(position(wrapper)).toBe('Change 1 of 2 · 2 left')
    expect(result.state.doc.line(2).text).toBe('o1 mine')
  })

  it('FR-CONF-10 clicking a conflict in the incoming pane selects it everywhere', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const incoming = viewAt(wrapper, INCOMING)

    incoming.dispatch({
      selection: { anchor: incoming.state.doc.line(6).from },
      userEvent: 'select.pointer',
    })
    await flushPromises()

    expect(position(wrapper)).toBe('Change 3 of 3 · 3 left')
    for (const pane of [CURRENT, RESULT, INCOMING] as const) {
      expect(selectedLineCount(viewAt(wrapper, pane))).toBeGreaterThan(0)
    }
    expect(selectedLineCount(viewAt(wrapper, RESULT))).toBe(1)

    press(incoming, { key: 'ArrowUp', altKey: true })
    await flushPromises()
    expect(position(wrapper)).toBe('Change 2 of 3 · 3 left')
  })

  it('FR-CONF-10 clicking a side conflict the restored result lacks leaves the selection alone and logs no error', async () => {
    const file = threeRegionFile()
    useMergeDrafts().set('/repo', 'a.txt', '', () => ({
      text: 'a\no1\nb\no2\nc\no3\nz',
      fingerprint: 'fp',
      regions: [{ range: { start: 2, lines: 1 }, resolved: false }],
    }))
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountEditor(file)
    const incoming = viewAt(wrapper, INCOMING)

    incoming.dispatch({
      selection: { anchor: incoming.state.doc.line(6).from },
      userEvent: 'select.pointer',
    })
    await flushPromises()

    expect(logged).not.toHaveBeenCalled()
    expect(position(wrapper)).toBe('Change 1 of 1 · 1 left')
    logged.mockRestore()
  })

  it('FR-CONF-10 the result cursor entering a conflict selects it', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const result = viewAt(wrapper, RESULT)

    result.dispatch({ selection: { anchor: result.state.doc.line(6).from } })
    await flushPromises()

    expect(position(wrapper)).toBe('Change 3 of 3 · 3 left')
  })

  it('FR-CONF-10 Alt+Down in the result moves to the next conflict, not the line', async () => {
    const wrapper = await mountEditor(threeRegionFile())
    const result = viewAt(wrapper, RESULT)

    press(result, { key: 'ArrowDown', altKey: true })
    await flushPromises()

    expect(position(wrapper)).toBe('Change 2 of 3 · 3 left')
    expect(result.state.doc.lineAt(result.state.selection.main.head).number).toBe(4)
    expect(result.state.doc.lines).toBe(7)
  })
})

const SAVE_AND_RESOLVE = 'Save and mark resolved'

describe('MergeEditor saving', () => {
  beforeEach(() => {
    Facade.SaveConflict.mockReset().mockResolvedValue('fp2')
  })

  it('FR-CONF-06 saving waits for the last conflict, and says so', async () => {
    const wrapper = await mountEditor()

    const resolve = findButton(wrapper, SAVE_AND_RESOLVE)!
    expect(resolve.attributes('disabled')).toBeDefined()
    expect(resolve.attributes('title')).toBe('Handle every change first')
    press(viewAt(wrapper, RESULT), { key: 's', ctrlKey: true })
    await flushPromises()

    expect(Facade.SaveConflict).not.toHaveBeenCalled()
  })

  it('FR-CONF-06 once every conflict is resolved the button and Ctrl+S save and mark resolved', async () => {
    const wrapper = await mountEditor()
    await click(wrapper, 'Take left')
    await click(wrapper, 'Next change · Alt+Down')
    await click(wrapper, 'Both, right first')

    expect(findButton(wrapper, SAVE_AND_RESOLVE)!.attributes('title')).toBe(
      'Save and mark resolved · Ctrl+S',
    )
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
      true,
    )
  })

  it('FR-CONF-06 any other save error shows as an error', async () => {
    Facade.SaveConflict.mockRejectedValue(new Error('disk full'))
    const wrapper = await mountEditor()
    await resolveBothConflicts(wrapper)

    await click(wrapper, SAVE_AND_RESOLVE)

    expect(wrapper.find('.error-banner').text()).toContain('disk full')
    expect(findButton(wrapper, 'Reload')).toBeUndefined()
  })
})

describe('MergeEditor toolbar and lifecycle', () => {
  it('FR-CONF-06 a stray marker line explains the disabled Save and mark resolved', async () => {
    const wrapper = await mountEditor()
    await resolveBothConflicts(wrapper)
    const result = viewAt(wrapper, RESULT)
    result.dispatch({ changes: { from: 0, insert: '>>>>>>> stray\n' } })
    await flushPromises()

    expect(position(wrapper)).toBe('No changes left')
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
  it('FR-CONF-11 typing during a save keeps the draft with the saved fingerprint', async () => {
    let finish: (fingerprint: string) => void = () => {}
    Facade.SaveConflict.mockReset().mockReturnValue(
      new Promise<string>((resolve) => (finish = resolve)),
    )
    const wrapper = await mountEditor()
    await resolveBothConflicts(wrapper)
    const result = viewAt(wrapper, RESULT)
    result.dispatch({ changes: { from: 0, insert: 'one ' } })
    await click(wrapper, SAVE_AND_RESOLVE)

    result.dispatch({ changes: { from: 0, insert: 'two ' } })
    finish('fpSaved')
    await flushPromises()

    const draft = useMergeDrafts().get('/repo', 'a.txt', '')
    expect(draft?.text).toMatch(/^two one a\n/)
    expect(draft?.fingerprint).toBe('fpSaved')
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

  it('FR-CONF-11 a result edited and left unsaved comes back when the file is shown again', async () => {
    const file = conflictFileFixture()
    const first = await mountEditor(file)
    await editResult(first)
    first.unmount()

    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)[RESULT]).toMatch(/^typed a\n/)
    expect(position(wrapper)).toBe('Change 1 of 2 · 2 left')
  })

  it('FR-CONF-11 resolved regions are still resolved when the file is shown again', async () => {
    const file = conflictFileFixture()
    const first = await mountEditor(file)
    await click(first, 'Take right')
    first.unmount()

    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)[RESULT]).toBe('a\ni1\nb\no2\nz')
    expect(position(wrapper)).toBe('Change 2 of 2 · 1 left')
    expect(viewAt(wrapper, RESULT).dom.querySelectorAll('.cm-conflict-resolved')).toHaveLength(1)
    expect(viewAt(wrapper, INCOMING).dom.querySelectorAll('.cm-conflict-resolved')).toHaveLength(1)
  })

  it('FR-CONF-11 a saved result leaves nothing to restore', async () => {
    const file = conflictFileFixture()
    const first = await mountEditor(file)
    await resolveBothConflicts(first)
    await click(first, SAVE_AND_RESOLVE)
    first.unmount()

    const wrapper = await mountEditor(file)

    expect(editorTexts(wrapper)[RESULT]).toBe('a\no1\nb\no2\nz')
    expect(useMergeDrafts().get('/repo', 'a.txt', '')).toBeUndefined()
  })

  it('FR-CONF-11 closing the editor writes nothing the core already has', async () => {
    vi.useFakeTimers()
    try {
      Facade.SetMergeDraft.mockClear()
      const wrapper = await mountEditor()
      viewAt(wrapper, RESULT).dispatch({ changes: { from: 0, insert: 'typed ' } })
      await vi.advanceTimersByTimeAsync(1000)

      wrapper.unmount()

      expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-CONF-11 an edit the core failed to keep shows the error', async () => {
    Facade.SetMergeDraft.mockRejectedValueOnce(new Error('no session'))
    const wrapper = await mountEditor()

    await editResult(wrapper)

    expect(wrapper.find('.error-banner').text()).toContain('no session')
  })
})

describe('MergeEditor changes only one side made', () => {
  it('FR-CONF-05 the result starts as the base text with every change open, tinted on the sides', async () => {
    const wrapper = await mountEditor(changesFileFixture())

    expect(viewAt(wrapper, RESULT).state.doc.toString()).toBe('a\no1\nb\no2\nc\no3\nz')
    expect(position(wrapper)).toBe('Change 1 of 4 · 4 left')
    expect(tintsOf(viewAt(wrapper, CURRENT))).toEqual([
      undefined,
      'cm-change-changed',
      undefined,
      'cm-conflict-unresolved',
      undefined,
      undefined,
      undefined,
    ])
    expect(tintsOf(viewAt(wrapper, INCOMING))).toEqual([
      undefined,
      undefined,
      undefined,
      'cm-conflict-unresolved',
      undefined,
      'cm-change-changed',
      undefined,
      'cm-change-added',
    ])
  })

  it.each([
    ['Take right', 'a\no1\nb\no2\nc\no3\nz'],
    ['Take left', 'a\nc1\nb\no2\nc\no3\nz'],
  ])(
    'FR-CONF-10 "%s" on a change the current side made resolves it in every pane',
    async (label, text) => {
      const wrapper = await mountEditor(changesFileFixture())

      await click(wrapper, label)

      expect(viewAt(wrapper, RESULT).state.doc.toString()).toBe(text)
      expect(position(wrapper)).toBe('Change 1 of 4 · 3 left')
      for (const pane of [CURRENT, RESULT, INCOMING] as const) {
        expect(viewAt(wrapper, pane).dom.querySelectorAll('.cm-conflict-resolved')).toHaveLength(1)
      }
    },
  )

  it('FR-CONF-10 Apply non-conflicting changes takes the changed side of every open one-side change in one undo step', async () => {
    const wrapper = await mountEditor(changesFileFixture())
    const result = viewAt(wrapper, RESULT)

    await click(wrapper, 'Apply non-conflicting changes')

    expect(result.state.doc.toString()).toBe('a\nc1\nb\no2\nc\ni3\nz\nnew')
    expect(position(wrapper)).toBe('Change 1 of 4 · 1 left')
    expect(
      findButton(wrapper, 'Apply non-conflicting changes')?.attributes('disabled'),
    ).toBeDefined()

    undo(result)
    await flushPromises()
    expect(result.state.doc.toString()).toBe('a\no1\nb\no2\nc\no3\nz')
    expect(position(wrapper)).toBe('Change 1 of 4 · 4 left')
  })

  it('FR-CONF-10 the selected change is outlined in every pane and marked in the gutter', async () => {
    const wrapper = await mountEditor(changesFileFixture())

    for (const pane of [CURRENT, RESULT, INCOMING] as const) {
      expect(markedLines(viewAt(wrapper, pane))).toEqual([2])
      expect(viewAt(wrapper, pane).dom.querySelectorAll('.cm-chunk-gutter-mark')).toHaveLength(1)
    }
    await click(wrapper, 'Next change · Alt+Down')
    for (const pane of [CURRENT, RESULT, INCOMING] as const) {
      expect(markedLines(viewAt(wrapper, pane))).toEqual([4])
    }
  })

  it('FR-CONF-06 saving waits until the changes only one side made are handled too', async () => {
    const wrapper = await mountEditor(changesFileFixture())
    await click(wrapper, 'Next change · Alt+Down')
    await click(wrapper, 'Take left')
    const save = () => findButton(wrapper, SAVE_AND_RESOLVE)!

    expect(save().attributes('disabled')).toBeDefined()
    expect(save().attributes('title')).toBe('Handle every change first')

    await click(wrapper, 'Apply non-conflicting changes')

    expect(save().attributes('disabled')).toBeUndefined()
  })
})
