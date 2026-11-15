<script setup lang="ts">
/**
 * Themed replacement for a native <select>. Browsers draw native option menus in the system
 * colours (Safari shows a dark menu over the light theme) and as wide as the field, so form
 * fields use this listbox instead. Values may be any type, including '' and numbers: items are
 * keyed by position internally because the listbox cannot hold an empty string.
 */
import { computed, ref } from 'vue';
import {
  SelectRoot,
  SelectTrigger,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectItem,
  SelectItemText,
  SelectItemIndicator,
} from 'reka-ui';
import AppIcon from './AppIcon.vue';

export type SelectOption = { value: unknown; label: string; disabled?: boolean };

defineOptions({ inheritAttrs: false });
const props = defineProps<{
  modelValue: unknown;
  options: SelectOption[];
  disabled?: boolean;
  placeholder?: string;
}>();
const emit = defineEmits<{ 'update:modelValue': [unknown]; change: [unknown] }>();

const index = computed(() => {
  let found = props.options.findIndex((o) => o.value === props.modelValue);
  if (found < 0 && props.modelValue != null)
    found = props.options.findIndex((o) => String(o.value) === String(props.modelValue));
  return found;
});
// A modal <dialog> makes everything outside it inert, so the menu opens inside it.
const trigger = ref<{ $el?: HTMLElement }>();
const host = ref<string | HTMLElement>('body');
function opened(open: boolean) {
  if (open) host.value = trigger.value?.$el?.closest('dialog') || 'body';
}
const chosen = computed(() => props.options[index.value]);
function pick(key: unknown) {
  const option = props.options[Number(key)];
  if (!option || option.value === props.modelValue) return;
  emit('update:modelValue', option.value);
  emit('change', option.value);
}
</script>
<template>
  <SelectRoot
    :model-value="index < 0 ? undefined : String(index)"
    :disabled="disabled"
    @update:model-value="pick"
    @update:open="opened"
    ><SelectTrigger ref="trigger" v-bind="$attrs" class="app-select-trigger"
      ><span :class="{ 'app-select-placeholder': !chosen }">{{
        chosen?.label || placeholder || 'Select an option'
      }}</span
      ><AppIcon name="down" :size="14" /></SelectTrigger
    ><SelectPortal :to="host"
      ><SelectContent
        class="scope-select-content app-select-content"
        position="popper"
        :side-offset="5"
        align="start"
        :collision-padding="12"
        ><SelectViewport
          ><SelectItem
            v-for="(option, i) in options"
            :key="i"
            class="scope-select-option"
            :value="String(i)"
            :disabled="option.disabled"
            ><SelectItemText>{{ option.label }}</SelectItemText
            ><SelectItemIndicator
              ><AppIcon
                name="check"
                :size="
                  14
                " /></SelectItemIndicator></SelectItem></SelectViewport></SelectContent></SelectPortal
  ></SelectRoot>
</template>
