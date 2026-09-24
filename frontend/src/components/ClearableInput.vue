<script setup lang="ts">
import { useI18n } from '../i18n'
import InputText from 'openvue/inputtext'
import IconField from 'openvue/iconfield'
import InputIcon from 'openvue/inputicon'
import { iconClass } from '../lib/icons'

// ClearableInput is a text field with a clear icon that shows once it has
// text: a plain InputText for FilterBar's message and path fields, since
// InputText has no showClear of its own.
defineProps<{ placeholder: string; testId: string }>()
const { t } = useI18n()
const emit = defineEmits<{ clear: [] }>()
const model = defineModel<string>({ default: '' })

function clear(): void {
  model.value = ''
  emit('clear')
}
</script>

<template>
  <IconField>
    <InputText v-model="model" :placeholder="placeholder" />
    <InputIcon v-if="model">
      <button
        type="button"
        class="clear-btn"
        :data-test="testId"
        :aria-label="t('common.clear')"
        :title="t('common.clear')"
        @click="clear"
      >
        <i :class="iconClass('clear')" aria-hidden="true" />
      </button>
    </InputIcon>
  </IconField>
</template>

<style scoped>
.clear-btn {
  display: inline-flex;
  padding: 0;
  border: none;
  background: none;
  color: inherit;
  cursor: pointer;
}
</style>
