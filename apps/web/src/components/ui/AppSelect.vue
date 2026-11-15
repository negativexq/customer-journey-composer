<script setup lang="ts">
/**
 * Themed replacement for a native <select>. Browsers draw native option menus in the system
 * colours (Safari shows a dark menu over the light theme) and as wide as the field, so form
 * fields use this listbox instead. Values may be any type, including '' and numbers: items are
 * keyed by position internally because the listbox cannot hold an empty string.
 *
 * Options can carry what the product already uses to tell things apart: a step icon, a status
 * tone, a one-line description and a group heading. Use SegmentedControl for two or three short
 * choices instead of this menu.
 */
import { computed, ref } from 'vue';
import {
  SelectRoot,
  SelectTrigger,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectGroup,
  SelectLabel,
  SelectItem,
  SelectItemText,
  SelectItemIndicator,
} from 'reka-ui';
import AppIcon from './AppIcon.vue';

export type SelectOption = {
  value: unknown;
  label: string;
  description?: string;
  icon?: string;
  tone?: 'green' | 'red' | 'amber' | 'blue' | 'purple' | 'neutral';
  group?: string;
  disabled?: boolean;
};

defineOptions({ inheritAttrs: false });
const props = defineProps<{
  modelValue: unknown;
  options: SelectOption[];
  disabled?: boolean;
  placeholder?: string;
  /** Small caption inside the trigger, for filters that sit without a field label. */
  caption?: string;
}>();
const emit = defineEmits<{ 'update:modelValue': [unknown]; change: [unknown] }>();

const index = computed(() => {
  let found = props.options.findIndex((o) => o.value === props.modelValue);
  if (found < 0 && props.modelValue != null)
    found = props.options.findIndex((o) => String(o.value) === String(props.modelValue));
  return found;
});
const chosen = computed(() => props.options[index.value]);
// Consecutive options that share a group are shown under one heading.
const sections = computed(() => {
  const list: { group?: string; items: { option: SelectOption; key: string }[] }[] = [];
  props.options.forEach((option, i) => {
    const last = list[list.length - 1];
    if (last && last.group === option.group) last.items.push({ option, key: String(i) });
    else list.push({ group: option.group, items: [{ option, key: String(i) }] });
  });
  return list;
});
function pick(key: unknown) {
  const option = props.options[Number(key)];
  if (!option || option.value === props.modelValue) return;
  emit('update:modelValue', option.value);
  emit('change', option.value);
}
// A modal <dialog> makes everything outside it inert, so the menu opens inside it.
const trigger = ref<{ $el?: HTMLElement }>();
const host = ref<string | HTMLElement>('body');
function opened(open: boolean) {
  if (open) host.value = trigger.value?.$el?.closest('dialog') || 'body';
}
</script>
<template>
  <SelectRoot
    :model-value="index < 0 ? undefined : String(index)"
    :disabled="disabled"
    @update:model-value="pick"
    @update:open="opened"
    ><SelectTrigger
      ref="trigger"
      v-bind="$attrs"
      class="app-select-trigger"
      :class="{ 'has-caption': caption }"
      ><AppIcon v-if="chosen?.icon" :name="chosen.icon" :size="15" class="app-select-icon" /><i
        v-else-if="chosen?.tone"
        class="app-select-dot"
        :class="'tone-' + chosen.tone" /><span class="app-select-value"
        ><small v-if="caption">{{ caption }}</small
        ><span :class="{ 'app-select-placeholder': !chosen }">{{
          chosen?.label || placeholder || 'Select an option'
        }}</span></span
      ><AppIcon name="down" :size="14" class="app-select-chevron" /></SelectTrigger
    ><SelectPortal :to="host"
      ><SelectContent
        class="app-select-content"
        position="popper"
        :side-offset="4"
        align="start"
        :collision-padding="12"
        ><SelectViewport
          ><SelectGroup v-for="(section, s) in sections" :key="s"
            ><SelectLabel v-if="section.group" class="app-select-group">{{
              section.group
            }}</SelectLabel
            ><SelectItem
              v-for="{ option, key } in section.items"
              :key="key"
              class="app-select-option"
              :class="{ 'has-description': option.description }"
              :value="key"
              :disabled="option.disabled"
              ><AppIcon
                v-if="option.icon"
                :name="option.icon"
                :size="15"
                class="app-select-icon" /><i
                v-else-if="option.tone"
                class="app-select-dot"
                :class="'tone-' + option.tone" /><span class="app-select-text"
                ><SelectItemText>{{ option.label }}</SelectItemText
                ><small v-if="option.description">{{ option.description }}</small></span
              ><SelectItemIndicator class="app-select-check"
                ><AppIcon
                  name="check"
                  :size="
                    14
                  " /></SelectItemIndicator></SelectItem></SelectGroup></SelectViewport></SelectContent></SelectPortal
  ></SelectRoot>
</template>
