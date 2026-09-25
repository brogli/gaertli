import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ErrorBanner from './ErrorBanner.vue'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { findButton } from '../testing/findButton'

describe('ErrorBanner', () => {
  it("FR-UX-01 an inline error is OpenVue's error Message", () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: Code.GitFailed, message: 'fatal: bad revision' } },
    })

    const message = wrapper.get('.p-message')
    expect(message.classes()).toContain('p-message-error')
    expect(message.text()).toContain('fatal: bad revision')
  })

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

  it('QA-INST-01 a too-old git, local or from a WSL repo, names its version and the minimum', () => {
    const wrapper = mount(ErrorBanner, {
      props: {
        error: {
          code: Code.GitTooOld,
          message: 'stale message, not what should be shown',
          key: 'gitTooOld',
          args: { version: '2.30.0', minVersion: '2.42.0' },
        },
      },
    })

    expect(wrapper.get('p').text()).toBe('git 2.30.0 is older than the minimum 2.42.0')
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

  it('FR-UX-16 a LOCKED error with a retry shows "Try again", running it on click', async () => {
    const retry = vi.fn()
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: Code.Locked, message: 'Another program is using the repo.', retry } },
    })

    const button = findButton(wrapper, 'Try again')
    expect(button).toBeDefined()
    await button!.trigger('click')

    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-16 an error with no retry shows no "Try again" button', () => {
    const wrapper = mount(ErrorBanner, {
      props: { error: { code: Code.GitFailed, message: 'fatal: bad revision' } },
    })

    expect(findButton(wrapper, 'Try again')).toBeUndefined()
  })
})
