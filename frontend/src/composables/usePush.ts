import { ref, toValue, type MaybeRefOrGetter, type Ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { useCoreCall } from './useCoreCall'

// UpstreamQuestion is the branch and remote a first push asks about;
// remote is '' when PushTarget could not name one, and the user must pick.
export interface UpstreamQuestion {
  branch: string
  remote: string
}

// usePush pushes the current branch, deciding from a fresh PushTarget alone
// (never the refs store, which a config-only change such as `git branch -u`
// does not refresh): hasUpstream pushes plainly; autoSetUpstream and a
// remote push the named branch straight away; otherwise upstreamQuestion is
// set for SetUpstreamDialog to ask and push.
export function usePush(
  wt: MaybeRefOrGetter<string>,
  run: ReturnType<typeof useCoreCall>['run'],
): { push: () => void; upstreamQuestion: Ref<UpstreamQuestion | null> } {
  const upstreamQuestion = ref<UpstreamQuestion | null>(null)

  function push(): void {
    upstreamQuestion.value = null
    void run(async () => {
      const target = await Facade.PushTarget(toValue(wt))
      if (target.hasUpstream) {
        await Facade.Push(toValue(wt), '', '')
      } else if (target.autoSetUpstream && target.remote) {
        await Facade.Push(toValue(wt), target.remote, target.branch)
      } else {
        upstreamQuestion.value = { branch: target.branch, remote: target.remote }
      }
    })
  }

  return { push, upstreamQuestion }
}
