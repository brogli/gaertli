<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import type { ChangedFileAction } from './changedFileAction'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'

// ChangedFileButton is one changed-file row's icon button, in the list and
// in the tree: kind names both its icon and its changesView.* label (Open
// in editor's is common.openInEditor), so the two views cannot show it out
// of step. The caller passes data-test and @click.stop through attrs.
const props = defineProps<{ kind: ChangedFileAction | 'openEditor' }>()
const { t } = useI18n()

const label = computed(() =>
  props.kind === 'openEditor' ? t('common.openInEditor') : t(`changesView.${props.kind}`),
)
</script>

<template>
  <Button v-bind="ICON_BUTTON" :icon="iconClass(kind)" :title="label" :aria-label="label" />
</template>
