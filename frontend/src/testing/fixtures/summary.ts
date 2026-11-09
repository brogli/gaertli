import {
  OperationKind,
  type Summary,
} from '../../../bindings/github.com/brogli/gaertli/internal/core'

// summaryFixture is a Summary answer.
export function summaryFixture(overrides: Partial<Summary> = {}): Summary {
  return {
    branch: 'main',
    detached: false,
    operation: OperationKind.$zero,
    editorName: 'editor',
    messageDraft: '',
    waitingStash: '',
    ...overrides,
  }
}
