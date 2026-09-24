import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ErrorBanner from './ErrorBanner.vue'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'

describe('ErrorBanner', () => {
  it("FR-UX-01, FR-ED-06 a git error's details show the command and git's full output", async () => {
    const wrapper = mount(ErrorBanner, {
      props: {
        error: {
          code: Code.GitFailed,
          message: "a branch named 'main' already exists",
          command: 'git branch main',
          gitStderr: "fatal: a branch named 'main' already exists\n",
        },
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.get('p').text()).toBe("a branch named 'main' already exists")
    expect(wrapper.get('pre').text()).toContain('git branch main')
    expect(wrapper.get('pre').text()).toContain("fatal: a branch named 'main' already exists")
  })

  it('FR-ED-06 an editor that cannot be started shows only the message naming the command', () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: Code.EditorNotFound, message: '"missing-editor" is not on PATH' } },
    })

    expect(wrapper.get('p').text()).toBe('"missing-editor" is not on PATH')
    expect(wrapper.findAll('button')).toEqual([])
  })

  it('FR-UX-11 an error with a known key shows the catalogue text with its args, not the message', () => {
    const wrapper = mount(ErrorBanner, {
      props: {
        error: {
          code: Code.NoUpstream,
          message: 'stale message, not what should be shown',
          key: 'noUpstream',
          args: { branch: 'main' },
        },
      },
    })

    expect(wrapper.get('p').text()).toBe('"main" has no upstream configured.')
  })

  it('FR-UX-11 an error with a key en.json has no entry for falls back to the message', () => {
    const wrapper = mount(ErrorBanner, {
      props: {
        error: {
          code: Code.GitFailed,
          message: 'the English fallback text',
          key: 'notInTheCatalogue',
        },
      },
    })

    expect(wrapper.get('p').text()).toBe('the English fallback text')
  })

  it('FR-UX-11 an internal error shows a fixed headline, the message as its detail', async () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: Code.Internal, message: 'nil pointer at worktree.go:42' } },
    })

    expect(wrapper.get('p').text()).not.toBe('nil pointer at worktree.go:42')
    await wrapper.get('button').trigger('click')
    expect(wrapper.get('pre').text()).toBe('nil pointer at worktree.go:42')
  })
})
