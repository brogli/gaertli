import { Operation, type Summary } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// summaryFixture is a Summary answer.
export function summaryFixture(overrides: Partial<Summary> = {}): Summary {
  return {
    branch: 'main',
    detached: false,
    operation: Operation.$zero,
    editorName: 'editor',
    messageDraft: '',
    pullStash: '',
    ...overrides,
  }
}
