<script setup lang="ts">
import { useI18n } from '../i18n'
import InputText from 'openvue/inputtext'
import IconField from 'openvue/iconfield'
import InputIcon from 'openvue/inputicon'
import { iconClass } from '../lib/icons'

// ClearableInput is a text field with a clear icon that shows once it has
// text: a plain InputText for FilterBar's message and path fields, since
// InputText has no showClear of its own. The icon clears on a click or
// Enter, as Select's and AutoComplete's own clear icons do on a click.
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
    <InputIcon
      v-if="model"
      :class="['clear', iconClass('clear')]"
      role="button"
      tabindex="0"
      :data-test="testId"
      :aria-label="t('common.clear')"
      :title="t('common.clear')"
      @click="clear"
      @keydown.enter="clear"
    />
  </IconField>
</template>

<style scoped>
.clear {
  cursor: pointer;
}
</style>
