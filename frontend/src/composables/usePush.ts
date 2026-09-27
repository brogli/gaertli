import { ref, toValue, type MaybeRefOrGetter, type Ref } from 'vue'
import { useRefsStore } from '@/stores/refs'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { useCoreCall } from './useCoreCall'

// UpstreamQuestion is the branch and remote a first push asks about;
// remote is '' when PushTarget could not name one, and the user must pick.
export interface UpstreamQuestion {
  branch: string
  remote: string
}

// usePush pushes the current branch. With an upstream, it pushes plainly.
// Without one, PushTarget decides: autoSetUpstream and a remote push
// straight away, otherwise upstreamQuestion is set for SetUpstreamDialog to
// ask and push.
export function usePush(
  wt: MaybeRefOrGetter<string>,
  run: ReturnType<typeof useCoreCall>['run'],
): { push: () => void; upstreamQuestion: Ref<UpstreamQuestion | null> } {
  const refs = useRefsStore()
  const upstreamQuestion = ref<UpstreamQuestion | null>(null)

  function push(): void {
    if (refs.current?.upstream) {
      void run(() => Facade.Push(toValue(wt), '', ''))
      return
    }
    upstreamQuestion.value = null
    void run(async () => {
      const target = await Facade.PushTarget(toValue(wt))
      if (target.autoSetUpstream && target.remote) {
        await Facade.Push(toValue(wt), target.remote, '')
      } else {
        upstreamQuestion.value = { branch: refs.current?.name ?? '', remote: target.remote }
      }
    })
  }

  return { push, upstreamQuestion }
}
