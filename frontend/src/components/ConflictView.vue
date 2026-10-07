<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { conflictSideNames } from '@/lib/conflictSides'
import { SECONDARY_BUTTON } from '@/lib/layout'
import { useRepoChanged } from '@/lib/windowEvents'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { useCoreCall, type CoreError } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  ChangeScope,
  ConflictChoice,
  ConflictKind,
  Operation,
  type ConflictFile,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'
import FileStepButtons from './FileStepButtons.vue'
import MergeEditor from './MergeEditor.vue'

// ConflictView is the diff window's right pane for a conflicted file: the
// file's conflict, read again whenever the index changes. A text file opens
// in the merge editor; any other kind gets a panel to pick a side or to
// resolve it elsewhere.
const props = defineProps<{ wt: string; path: string }>()
const emit = defineEmits<{ 'step-file': [step: 1 | -1] }>()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { confirmAction } = useConfirmAction()
const { t } = useI18n()
const drafts = useMergeDrafts()

const conflict = ref<ConflictFile | null>(null)

// generation changes to mount the merge editor afresh.
const generation = ref(0)

// version counts reads and saves; a read answered after a newer read or a
// save is stale and dropped.
let version = 0

// load reads the conflict. A file that changed on disk is shown afresh,
// unless the merge editor holds unsaved edits: it keeps them and says so.
async function load(): Promise<void> {
  const started = ++version
  // A stale read's failure is moot, and a file that is no longer conflicted
  // is left by the window itself.
  const ignoreError = (e: CoreError) => started !== version || e.key === 'notConflicted'
  const next = (await run(() => Facade.Conflict(props.wt, props.path), ignoreError)) ?? null
  if (started !== version) return
  const conflictId = next?.conflictId ?? ''
  if (next) await run(() => drafts.load(props.wt, props.path, conflictId), ignoreError)
  if (started !== version) return
  const shown = conflict.value
  const samePath = next?.path === shown?.path
  const changed = samePath && next?.fingerprint !== shown?.fingerprint
  const newConflict = samePath && conflictId !== (shown?.conflictId ?? '')
  if (newConflict || (changed && !drafts.has(props.wt, props.path, conflictId))) generation.value++
  conflict.value = next
}

// reload drops the merge editor's unsaved edits for the file as it is on
// disk.
async function reload(): Promise<void> {
  await drafts.discard(props.wt, props.path)
  await load()
  generation.value++
}

// saved takes a save's fingerprint as the shown file's, so the reload the save
// causes keeps the editor.
function saved(fingerprint: string): void {
  if (conflict.value?.path !== props.path) return
  version++
  conflict.value = { ...conflict.value, fingerprint }
}

watch(
  () => [props.wt, props.path],
  () => void load(),
  { immediate: true },
)
useRepoChanged([ChangeScope.IndexChanged], () => void load())

const plainNames = computed(() =>
  conflict.value
    ? conflictSideNames(
        conflict.value.operation,
        conflict.value.current,
        conflict.value.incoming,
        t,
      )
    : { current: '', incoming: '' },
)

const SIDE_PICK_KINDS: (ConflictKind | undefined)[] = [
  ConflictKind.ConflictDeleted,
  ConflictKind.ConflictBinary,
  ConflictKind.ConflictSymlink,
]
const picksSide = computed(() => SIDE_PICK_KINDS.includes(conflict.value?.kind))

// versions are the buttons that keep one side's version: one per side that
// has one.
const versions = computed(() => {
  const file = conflict.value
  if (!file) return []
  return [
    {
      choice: ConflictChoice.ChooseCurrent,
      present: file.current.present,
      label: t('conflictView.keepVersion', { side: plainNames.value.current }),
    },
    {
      choice: ConflictChoice.ChooseIncoming,
      present: file.incoming.present,
      label:
        file.operation === Operation.RevertOperation
          ? t('conflictView.takeVersionBefore', { commit: plainNames.value.incoming })
          : t('conflictView.takeVersion', { side: plainNames.value.incoming }),
    },
  ].filter((v) => v.present)
})

const reason = computed(() => {
  const file = conflict.value
  if (!file) return ''
  if (file.kind !== ConflictKind.ConflictDeleted) return t(`conflictView.${file.kind}`)
  const currentDeleted = !file.current.present
  return t('conflictView.deleted', {
    deleter: currentDeleted ? plainNames.value.current : plainNames.value.incoming,
    other: currentDeleted ? plainNames.value.incoming : plainNames.value.current,
  })
})

function resolve(choice: ConflictChoice): void {
  void run(() => Facade.ResolveConflict(props.wt, props.path, choice))
}

function confirmDelete(): void {
  confirmAction(
    t('conflictView.deleteFile'),
    t('conflictView.deleteConfirm', { path: props.path }),
    () => resolve(ConflictChoice.ChooseDelete),
  )
}
</script>

<template>
  <div class="conflict-view h-full min-h-0">
    <ErrorBanner v-if="error" :error="error" class="m-2.5" />
    <MergeEditor
      v-else-if="conflict?.kind === ConflictKind.ConflictText"
      :key="`${wt}\n${conflict.path}\n${generation}`"
      class="p-2.5"
      :wt="wt"
      :file="conflict"
      @step-file="emit('step-file', $event)"
      @reload="reload"
      @saved="saved"
    />
    <section v-else-if="conflict" class="flex flex-col gap-2 p-2.5">
      <div class="flex items-center gap-1">
        <FileStepButtons @step="emit('step-file', $event)" />
      </div>
      <h2 class="truncate" :title="path">{{ path }}</h2>
      <p>{{ reason }}</p>
      <div v-if="picksSide" class="flex flex-wrap gap-1.5">
        <Button
          v-for="v in versions"
          :key="v.choice"
          v-bind="SECONDARY_BUTTON"
          :label="v.label"
          @click="resolve(v.choice)"
        />
        <Button
          v-bind="SECONDARY_BUTTON"
          :label="t('conflictView.deleteFile')"
          @click="confirmDelete"
        />
      </div>
      <div v-else class="flex flex-wrap gap-1.5">
        <Button
          v-bind="SECONDARY_BUTTON"
          :label="t('common.openInEditor')"
          @click="openEditor({ path })"
        />
        <Button
          v-bind="SECONDARY_BUTTON"
          :label="t('changesView.markResolved')"
          @click="run(() => Facade.StageFile(wt, path))"
        />
      </div>
    </section>
  </div>
</template>
