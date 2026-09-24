import type { JobInfo } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// jobFixture is a running fetch job.
export function jobFixture(overrides: Partial<JobInfo> = {}): JobInfo {
  return { id: 'j1', kind: 'fetch', state: 'running', progress: 0, message: '', ...overrides }
}
