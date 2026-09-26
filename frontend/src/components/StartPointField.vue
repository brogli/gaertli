<script setup lang="ts">
import { computed } from 'vue'
import InputText from 'openvue/inputtext'
import Select from 'openvue/select'
import { useI18n } from '@/i18n'
import { refGroups } from '@/lib/refOptions'
import { useRefsStore } from '@/stores/refs'
import FormField from './FormField.vue'
import type { StartPoint } from './startPoint'

// StartPointField is a new branch's start point: a fixed commit shown by its
// label, or a branch or tag the user picks from the window's refs.
const start = defineModel<StartPoint>({ required: true })
const refs = useRefsStore()
const { t } = useI18n()

const options = computed(() => refGroups(refs.refs, t))
const picked = computed({
  get: () => ('ref' in start.value ? start.value.ref : ''),
  set: (ref: string) => (start.value = { ref }),
})
</script>

<template>
  <FormField v-slot="{ id }" :label="t('dialogs.startPoint')">
    <InputText v-if="'rev' in start" :id="id" :model-value="start.label" disabled fluid />
    <Select
      v-else
      v-model="picked"
      :options="options"
      option-label="label"
      option-value="value"
      option-group-label="label"
      option-group-children="items"
      :input-id="id"
      :placeholder="t('dialogs.pickStart')"
      filter
      auto-filter-focus
      fluid
      data-test="start-branch"
    />
  </FormField>
</template>
