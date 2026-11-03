// Requirements: FR-CONF-05
import { describe, expect, it } from 'vitest'
import { EditorState } from '@codemirror/state'
import { codeEditorExtensions } from '../codeEditor'

describe('codeEditorExtensions', () => {
  it('splits lines at "\\n" only', async () => {
    const state = EditorState.create({
      doc: 'a\rb\nc',
      extensions: await codeEditorExtensions('x'),
    })
    expect(state.doc.lines).toBe(2)
  })
})
