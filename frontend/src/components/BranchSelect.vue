<script setup lang="ts">
import { computed } from 'vue'
import Select from 'openvue/select'
import { useI18n } from '../i18n'
import { useRefsStore } from '../stores/refs'

// BranchSelect picks a local branch by name from a searchable list.
defineProps<{ inputId: string }>()
const branch = defineModel<string>({ required: true })
const refs = useRefsStore()
const { t } = useI18n()

const options = computed(() => (refs.refs?.branches ?? []).map((b) => b.name))
</script>

<template>
  <Select
    v-model="branch"
    :options="options"
    :input-id="inputId"
    :placeholder="t('dialogs.pickBranch')"
    filter
    auto-filter-focus
    fluid
  />
</template>
