<script setup lang="ts">
import { computed, ref } from 'vue';
import { pathValue, setPath, type InputField } from '../../lib/test-context';
const props = defineProps<{
  modelValue: string;
  fields: InputField[];
  root: 'customer' | 'event';
}>();
const emit = defineEmits<{ 'update:modelValue': [string] }>();
const error = ref('');
const object = computed(() => {
  try {
    return JSON.parse(props.modelValue) as Record<string, unknown>;
  } catch {
    return {};
  }
});
const relative = (path: string) => path.replace(new RegExp('^' + props.root + '\\.'), '');
function change(field: InputField, event: Event) {
  try {
    const data = JSON.parse(props.modelValue);
    const input = event.target as HTMLInputElement;
    setPath(
      data,
      relative(field.path),
      field.type === 'boolean'
        ? input.checked
        : field.type === 'number'
          ? Number(input.value)
          : input.value,
    );
    emit('update:modelValue', JSON.stringify(data, null, 2));
    error.value = '';
  } catch {
    error.value = 'Correct the advanced JSON before editing these values.';
  }
}
</script>
<template>
  <div v-if="fields.length" class="scenario-fields">
    <label
      v-for="field in fields"
      :key="field.path"
      :class="{ 'checkbox-label': field.type === 'boolean' }"
      ><span
        >{{
          field.path
            .replace(/^customer\.attributes\.|^event\.properties\./, '')
            .replaceAll('_', ' ')
        }}<small :title="field.path">{{ field.source }}</small></span
      ><input
        :type="field.type === 'boolean' ? 'checkbox' : field.type === 'number' ? 'number' : 'text'"
        :value="pathValue(object, relative(field.path))"
        :checked="!!pathValue(object, relative(field.path))"
        :aria-label="`Sample ${field.path}`"
        @input="field.type !== 'boolean' && change(field, $event)"
        @change="field.type === 'boolean' && change(field, $event)"
    /></label>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  </div>
</template>
