// Requirements: FR-SYNC-06
import { describe, expect, it } from 'vitest'
import type { Refs } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from '@/testing/fixtures/branch'
import { branchSuggestions } from '../refOptions'

const refs: Refs = {
  branches: ['main', 'master', 'feature/x'].map((name) => branchFixture({ name })),
  remotes: [{ name: 'origin', branches: [branchFixture({ name: 'main' })] }],
  tags: [],
}

describe('branchSuggestions', () => {
  it.each([
    [
      'matching branches without duplicates, then the typed text',
      'ma',
      [],
      ['main', 'master', 'ma'],
    ],
    ['the typed text is the only suggestion when no branch matches', 'release', [], ['release']],
    ['a chosen branch is not suggested again', 'ma', ['main'], ['master', 'ma']],
    ['a typed name that is already chosen is not suggested', 'master', ['master'], []],
  ])('%s', (_name, query, chosen, expected) => {
    expect(branchSuggestions(refs, query, chosen)).toEqual(expected)
  })

  it('outside a repo, only the typed text is suggested', () => {
    expect(branchSuggestions(null, 'dev', [])).toEqual(['dev'])
  })
})
