import type { Summary } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// summaryFixture is a Summary answer; a Summary shape change is updated
// here once, not per test file.
export function summaryFixture(overrides: Partial<Summary> = {}): Summary {
  return {
    path: '/repo',
    branch: 'main',
    detached: false,
    operation: '',
    editorName: 'editor',
    messageDraft: '',
    pullStash: '',
    ...overrides,
  }
}
