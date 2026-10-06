<script setup lang="ts">
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'

// FileStepButtons are the previous and next file buttons of the diff
// window's conflict view; the parent holds the file list.
const emit = defineEmits<{ step: [step: 1 | -1] }>()
const { t } = useI18n()

const buttons = [
  { test: 'prev-file', label: 'diffWindow.previousFile', icon: 'prevFile', step: -1 },
  { test: 'next-file', label: 'diffWindow.nextFile', icon: 'nextFile', step: 1 },
] as const
</script>

<template>
  <Button
    v-for="b in buttons"
    :key="b.test"
    v-bind="ICON_BUTTON"
    :icon="iconClass(b.icon)"
    :data-test="b.test"
    :title="t(b.label)"
    :aria-label="t(b.label)"
    @click="emit('step', b.step)"
  />
</template>
