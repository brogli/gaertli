import type { VueWrapper } from '@vue/test-utils'
import { findButton } from '@/testing/findButton'

// resolveBothConflicts takes the left side in both conflicts of
// conflictFileFixture's file, stepping from the first to the second.
export async function resolveBothConflicts(wrapper: VueWrapper): Promise<void> {
  for (const label of ['Take left', 'Next change · Alt+Down', 'Take left'])
    await findButton(wrapper, label)!.trigger('click')
}
