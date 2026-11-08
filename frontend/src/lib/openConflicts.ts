import { AppService, Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { Code } from '../../bindings/github.com/brogli/gaertli/internal/core'
import type { CoreError } from '@/composables/useCoreCall'
import { DiffKind, fileTarget } from './diffTarget'

// openConflicts opens the diff window on wt's first conflicted file, read
// fresh: the stores may not yet show the stop that calls it.
export async function openConflicts(wt: string): Promise<void> {
  const [first] = (await Facade.Status(wt)).conflicted ?? []
  if (!first) return
  await AppService.OpenDiff(wt, fileTarget(first, { kind: DiffKind.WorkingDiff, conflicted: true }))
}

// openOnConflicts is a call's error handler: a stop on conflicts opens the
// diff window and counts as handled; any other error is left to the caller.
export function openOnConflicts(wt: string): (error: CoreError) => boolean {
  return (error) => {
    if (error.code !== Code.Conflicts) return false
    void openConflicts(wt)
    return true
  }
}
