<script setup lang="ts">
import { computed } from 'vue';
import FilterSelect from './FilterSelect.vue';
import AppIcon from './AppIcon.vue';
defineProps<{ modelValue: number; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [number] }>();
function range(days: number) {
  const end = new Date();
  const start = new Date(
    Date.UTC(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate()) - (days - 1) * 86400000,
  );
  const format = (date: Date) =>
    date.toLocaleDateString('en', { month: 'short', day: 'numeric', timeZone: 'UTC' });
  return `${format(start)} – ${format(end)}, ${end.getUTCFullYear()}`;
}
const options = computed(() =>
  [7, 30, 90].map((days) => ({
    value: String(days),
    label: `Last ${days} days`,
    description: range(days),
  })),
);
</script>
<template>
  <FilterSelect
    :model-value="String(modelValue)"
    label="Trend period"
    :options="options"
    icon="calendar"
    :disabled="disabled"
    @update:model-value="emit('update:modelValue', Number($event))"
  >
    <template #footer
      ><div class="filter-footer">
        <AppIcon name="clock" :size="13" /><span>Includes today · dates use UTC</span>
      </div></template
    >
  </FilterSelect>
</template>
