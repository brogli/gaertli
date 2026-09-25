<script setup lang="ts">
import { useId } from 'vue'

// FormField is one row of a form: a label above its control. The slot gets
// id, for the control's id (or inputId), and labelId, for a control named by
// aria-labelledby instead.
defineProps<{ label: string }>()
const id = useId()
const labelId = `${id}-label`
</script>

<template>
  <div class="form-field">
    <label :id="labelId" :for="id">{{ label }}</label>
    <slot v-bind="{ id, labelId }" />
  </div>
</template>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
</style>
