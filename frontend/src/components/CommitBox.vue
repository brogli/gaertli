<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import Textarea from 'openvue/textarea'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useChangesStore } from '../stores/changes'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// CommitBox is the message box at the top of the changes column: a
// message, an Amend checkbox, and a Commit button gated by the staged
// count — amend alone needs nothing staged.
const props = defineProps<{ wt: string }>()
const changes = useChangesStore()
const repo = useRepoStore()
const refs = useRefsStore()
const { error, run, runAndClose } = useCoreCall()

const message = ref('')
const amend = ref(false)
const committing = ref(false)

// Seed once from the session's own draft when Summary first carries one,
// never again afterward, so a later Summary reload does not clobber what
// this window's own textarea currently holds.
let draftLoaded = false
// The last value read from or written to the stored draft, so seeding the
// textarea from it does not itself trigger a save of the same text back.
let lastSyncedDraft: string | undefined
watch(
  () => repo.summary?.messageDraft,
  (draft) => {
    if (draftLoaded || draft === undefined) return
    message.value = draft
    lastSyncedDraft = draft
    draftLoaded = true
  },
  { immediate: true },
)

// 300ms matches FilterBar's debounce.
const saveDraft = useDebounceFn((text: string) => {
  lastSyncedDraft = text
  void Facade.SetMessageDraft(props.wt, text)
}, 300)
watch(message, (text) => {
  if (text !== lastSyncedDraft) void saveDraft(text)
})
onBeforeUnmount(() => void Facade.SetMessageDraft(props.wt, message.value))

const stagedCount = computed(() => changes.files?.staged?.length ?? 0)
const commitDisabled = computed(
  () => committing.value || !message.value.trim() || (!amend.value && stagedCount.value === 0),
)
const commitLabel = computed(() => (amend.value ? 'Amend' : `Commit${stagedCount.value ? ` (${stagedCount.value})` : ''}`))
// git itself refuses to amend mid-merge/rebase/etc.
const amendDisabled = computed(() => !repo.summary?.sha || !!repo.summary?.operation)

// Amending an already-pushed commit needs a force push. Read from the
// refs store's own ahead count instead of a fresh merge-base call: the
// current branch is an ancestor-or-equal of its upstream exactly when
// Ahead == 0.
const pushedWarning = computed(() => {
  if (!amend.value) return false
  const branch = refs.refs?.branches?.find((b) => b.current)
  return !!branch?.upstream && !branch.upstreamGone && branch.ahead === 0
})

async function onAmendChange(checked: boolean): Promise<void> {
  // Pre-fill from HEAD's own message only when the field is still empty:
  // never clobber a typed or restored draft.
  const sha = repo.summary?.sha
  if (checked && !message.value.trim() && sha) {
    const details = await run(() => Facade.CommitDetails(props.wt, sha))
    if (details) message.value = details.body
  }
}

async function onCommit(): Promise<void> {
  committing.value = true
  try {
    await runAndClose(
      () => Facade.Commit(props.wt, message.value, amend.value),
      () => {
        message.value = ''
        amend.value = false
      },
    )
  } finally {
    committing.value = false
  }
}
</script>

<template>
  <section class="commit-box">
    <ErrorBanner v-if="error" :error="error" />
    <Textarea v-model="message" rows="4" auto-resize placeholder="Commit message" />
    <label>
      <Checkbox v-model="amend" binary :disabled="amendDisabled" @update:model-value="onAmendChange" />
      Amend last commit
    </label>
    <p v-if="pushedWarning" class="warning">
      This commit has already been pushed. Amending will require a force push.
    </p>
    <Button :label="commitLabel" :disabled="commitDisabled" data-test="commit-submit" @click="onCommit" />
  </section>
</template>
