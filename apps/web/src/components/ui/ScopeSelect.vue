<script setup lang="ts">
import {
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectItem,
  SelectItemText,
  SelectItemIndicator,
} from 'reka-ui';
import AppIcon from './AppIcon.vue';
defineProps<{
  modelValue: string;
  label: string;
  options: { value: string; label: string }[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [string] }>();
</script>
<template>
  <SelectRoot
    :model-value="modelValue || 'all'"
    :disabled="disabled"
    @update:model-value="emit('update:modelValue', String($event))"
    ><SelectTrigger class="scope-select-trigger" :aria-label="label"
      ><span
        ><small>{{ label }}</small
        ><SelectValue>{{
          options.find((option) => option.value === (modelValue || 'all'))?.label
        }}</SelectValue></span
      ><AppIcon name="down" :size="14" /></SelectTrigger
    ><SelectPortal
      ><SelectContent class="scope-select-content" position="popper" :side-offset="6" align="start"
        ><SelectViewport
          ><SelectItem
            v-for="option in options"
            :key="option.value"
            class="scope-select-option"
            :value="option.value"
            ><SelectItemText>{{ option.label }}</SelectItemText
            ><SelectItemIndicator
              ><AppIcon
                name="check"
                :size="
                  14
                " /></SelectItemIndicator></SelectItem></SelectViewport></SelectContent></SelectPortal
  ></SelectRoot>
</template>
