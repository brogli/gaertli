import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    PickAndOpen: vi.fn(),
    RecentRepos: vi.fn().mockResolvedValue([]),
    OpenRecent: vi.fn(),
  },
}))

import App from './App.vue'

describe('App', () => {
  it('QA-OSS-02 renders the app name with no worktree open', () => {
    expect(mount(App).text()).toContain('nebegit')
  })
})
