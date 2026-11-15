<script setup lang="ts">
/**
 * Two to four short, mutually exclusive choices shown side by side, so the options are visible
 * without opening a menu. Arrow keys move the choice, as in a native radio group.
 */
import { ref } from 'vue';
import AppIcon from './AppIcon.vue';

export type SegmentOption = { value: unknown; label: string; icon?: string };

const props = defineProps<{
  modelValue: unknown;
  options: SegmentOption[];
  /** Accessible name; shown above the control unless hideLabel is set. */
  label: string;
  hideLabel?: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [unknown]; change: [unknown] }>();
const buttons = ref<HTMLButtonElement[]>([]);
const id = 'segment-' + crypto.randomUUID();

const isChosen = (value: unknown) =>
  value === props.modelValue ||
  (props.modelValue != null && String(value) === String(props.modelValue));
function pick(value: unknown) {
  if (props.disabled || isChosen(value)) return;
  emit('update:modelValue', value);
  emit('change', value);
}
function move(event: KeyboardEvent, i: number) {
  const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
  if (!step) return;
  event.preventDefault();
  const next = (i + step + props.options.length) % props.options.length;
  pick(props.options[next].value);
  buttons.value[next]?.focus();
}
</script>
<template>
  <div class="segmented-field">
    <span v-if="!hideLabel" :id="id" class="field-label">{{ label }}</span>
    <div
      class="segmented-control"
      role="radiogroup"
      :aria-label="hideLabel ? label : undefined"
      :aria-labelledby="hideLabel ? undefined : id"
      :aria-disabled="disabled || undefined"
    >
      <button
        v-for="(option, i) in options"
        :key="String(option.value)"
        ref="buttons"
        type="button"
        role="radio"
        :aria-checked="isChosen(option.value)"
        :tabindex="
          isChosen(option.value) || (i === 0 && !options.some((o) => isChosen(o.value))) ? 0 : -1
        "
        :disabled="disabled"
        @click="pick(option.value)"
        @keydown="move($event, i)"
      >
        <AppIcon v-if="option.icon" :name="option.icon" :size="14" />{{ option.label }}
      </button>
    </div>
  </div>
</template>
