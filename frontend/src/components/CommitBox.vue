<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useI18n } from '@/i18n'
import Textarea from 'openvue/textarea'
import Checkbox from 'openvue/checkbox'
import Message from 'openvue/message'
import Button from 'openvue/button'
import { useChangesStore } from '@/stores/changes'
import { useWorktreesStore } from '@/stores/worktrees'
import { useBranchesStore } from '@/stores/branches'
import { useCoreCall } from '@/composables/useCoreCall'
import { usePush } from '@/composables/usePush'
import { useShortcut } from '@/composables/useShortcut'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import ErrorBanner from './ErrorBanner.vue'
import SetUpstreamDialog from './SetUpstreamDialog.vue'
import ShortcutHint from './ShortcutHint.vue'

// CommitBox is the commit box, the changes view's fixed part below the
// change list: the message, Amend, Push after commit, and a Commit button
// gated by the staged count — amend alone needs nothing staged.
const props = defineProps<{ wt: string }>()
const changes = useChangesStore()
const repo = useWorktreesStore()
const refs = useBranchesStore()
const { error, run, runAndClose } = useCoreCall()
const { push, upstreamQuestion } = usePush(() => props.wt, run)
const { t } = useI18n()

// The message lives in the changes store, where the window finds it on a
// core restart; each commit box (one per worktree) starts it empty.
const { message } = storeToRefs(changes)
message.value = ''
const amend = ref(false)
const pushAfterCommit = ref(false)
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
const commitLabel = computed(() =>
  amend.value
    ? t('changesView.commitBox.amendButton')
    : stagedCount.value
      ? t('changesView.commitBox.commitCount', { n: stagedCount.value })
      : t('changesView.commitBox.commit'),
)
// git itself refuses to amend mid-merge/rebase/etc.
const amendDisabled = computed(() => !repo.summary?.sha || !!repo.summary?.operation)

// Amending an already-pushed commit needs a force push. Read from the
// refs store's own ahead count instead of a fresh merge-base call: the
// current branch is an ancestor-or-equal of its upstream exactly when
// Ahead == 0.
const pushedWarning = computed(() => {
  if (!amend.value) return false
  const branch = refs.current
  return !!branch?.upstream && !branch.upstreamGone && branch.ahead === 0
})

// "Push after commit" is off while amending (a plain push of an amended,
// pushed commit is rejected; pushedWarning names the force push it needs),
// on a detached HEAD, and without a remote. Not gated on an upstream: usePush
// gives a branch without one a first push instead.
const hasRemote = computed(() => (refs.refs?.remotes ?? []).length > 0)
const pushAfterCommitDisabled = computed(() => amend.value || !refs.current || !hasRemote.value)

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
  // Read before runAndClose's own onSuccess resets amend, which would
  // otherwise clear pushAfterCommitDisabled along with it and push after
  // an amend.
  const pushAfter = pushAfterCommit.value && !pushAfterCommitDisabled.value
  try {
    await runAndClose(
      () => Facade.Commit(props.wt, message.value, amend.value),
      () => {
        message.value = ''
        amend.value = false
      },
    )
    // A push failure after a successful commit shows in the same ErrorBanner.
    if (!error.value && pushAfter) push()
  } finally {
    committing.value = false
  }
}

// Ctrl+K is gated the same as the Commit button: a shortcut never bypasses
// the button's disabled state.
useShortcut(
  (event) => event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 'k',
  () => {
    if (!commitDisabled.value) void onCommit()
  },
)
</script>

<template>
  <section class="commit-box flex flex-col gap-2">
    <ErrorBanner v-if="error" :error="error" />
    <Textarea
      v-model="message"
      rows="6"
      fluid
      auto-resize
      :placeholder="t('changesView.commitBox.messagePlaceholder')"
    />
    <label class="flex items-center gap-1.5">
      <Checkbox
        v-model="amend"
        binary
        :disabled="amendDisabled"
        @update:model-value="onAmendChange"
      />
      {{ t('changesView.commitBox.amend') }}
    </label>
    <Message v-if="pushedWarning" severity="warn">
      {{ t('changesView.commitBox.pushedWarning') }}
    </Message>
    <label class="flex items-center gap-1.5">
      <Checkbox
        v-model="pushAfterCommit"
        binary
        :disabled="pushAfterCommitDisabled"
        data-test="push-after-commit"
      />
      {{ t('changesView.commitBox.pushAfterCommit') }}
    </label>
    <Button
      :label="commitLabel"
      :disabled="commitDisabled"
      data-test="commit-submit"
      @click="onCommit"
    >
      {{ commitLabel }}
      <ShortcutHint :shortcut="t('shortcuts.commit')" />
    </Button>
    <SetUpstreamDialog
      v-if="upstreamQuestion"
      :wt="wt"
      v-bind="upstreamQuestion"
      @close="upstreamQuestion = null"
    />
  </section>
</template>
