import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: { CompareCommits: vi.fn(), CompareWithWorking: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useLogStore } from './log'
import { useCompareStore } from './compare'
import { commitRow } from '../testing/fixtures/log-rows'

const row = (sha: string) => commitRow({ sha, shortSha: sha.slice(0, 7) })

describe('compare store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CompareCommits.mockReset()
    Facade.CompareWithWorking.mockReset()
  })

  it('FR-CMP-01 from is the commit selected first, to the one selected second, whatever their dates; three or more form no comparison', () => {
    const log = useLogStore()
    const compare = useCompareStore()
    const pickedFirst = row('picked-first') // the later-dated commit, selected first
    const pickedSecond = row('picked-second') // the earlier-dated commit, selected second

    log.selection = [pickedFirst, pickedSecond]
    expect(compare.from).toEqual(pickedFirst)
    expect(compare.to).toEqual(pickedSecond)
    expect(compare.active).toBe(true)

    log.selection = [pickedFirst, pickedSecond, row('third')]
    expect(compare.active).toBe(false)
  })

  it('FR-CMP-03 swap exchanges from and to, so the log view marks move to the other commit', () => {
    const log = useLogStore()
    const compare = useCompareStore()
    const pickedFirst = row('picked-first')
    const pickedSecond = row('picked-second')
    log.selection = [pickedFirst, pickedSecond]

    compare.swap()

    expect(compare.from).toEqual(pickedSecond)
    expect(compare.to).toEqual(pickedFirst)
  })

  it('FR-CMP-04 startAgainstWorking sets workingCommit and clears any selection', () => {
    const log = useLogStore()
    const compare = useCompareStore()
    log.selection = [row('x'), row('y')]

    const rowZ = row('z')
    compare.startAgainstWorking(rowZ)

    expect(compare.workingCommit).toEqual(rowZ)
    expect(log.selection).toEqual([])
  })

  it('FR-CMP-04 loadAgainstWorking populates files from Facade.CompareWithWorking', async () => {
    const compare = useCompareStore()
    const files = [{ status: 'M', path: 'a.txt' }]
    Facade.CompareWithWorking.mockResolvedValue(files)

    await compare.loadAgainstWorking('/repo', 'sha1')

    expect(Facade.CompareWithWorking).toHaveBeenCalledWith('/repo', 'sha1')
    expect(compare.files).toEqual(files)
  })

  it('FR-CMP-01 exit drops the selection and any comparison state', () => {
    const log = useLogStore()
    const compare = useCompareStore()
    log.selection = [row('x'), row('y')]
    compare.selectFile({ status: 'M', path: 'a.txt' })

    compare.exit()

    expect(log.selection).toEqual([])
    expect(compare.selectedFile).toBeNull()
    expect(compare.workingCommit).toBeNull()
  })
})
