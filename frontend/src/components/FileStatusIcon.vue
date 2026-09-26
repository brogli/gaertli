<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { oiClass, statusIcon, TONE_CLASS } from '@/lib/icons'

// FileStatusIcon is a changed file's status as an icon in its tone, the
// status named in its tooltip.
const props = defineProps<{ status: string }>()
const { t } = useI18n()
const icon = computed(() => statusIcon(props.status, t))
</script>

<template>
  <span
    class="status flex-none w-[1.2em] text-center"
    :class="TONE_CLASS[icon.tone]"
    role="img"
    :aria-label="icon.label"
    :title="icon.label"
  >
    <i :class="oiClass(icon.icon)"></i>
  </span>
</template>
