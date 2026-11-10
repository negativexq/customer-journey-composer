<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import AppIcon from './AppIcon.vue';
const props = defineProps<{
  modelValue: string;
  label: string;
  options: { value: string; label: string; description?: string }[];
  icon?: string;
  searchable?: boolean;
  searchPlaceholder?: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [string] }>();
const root = ref<HTMLElement>();
const trigger = ref<HTMLButtonElement>();
const search = ref<HTMLInputElement>();
const list = ref<HTMLElement>();
const open = ref(false);
const query = ref('');
const active = ref(0);
const id = 'filter-' + crypto.randomUUID();
const filtered = computed(() =>
  props.options.filter((o) =>
    o.label.toLocaleLowerCase().includes(query.value.toLocaleLowerCase()),
  ),
);
const chosen = computed(() => props.options.find((o) => o.value === props.modelValue));
async function show() {
  if (props.disabled) return;
  open.value = true;
  query.value = '';
  active.value = Math.max(
    0,
    props.options.findIndex((o) => o.value === props.modelValue),
  );
  await nextTick();
  (props.searchable ? search.value : list.value)?.focus();
  scroll();
}
function close(restore = false) {
  open.value = false;
  if (restore) trigger.value?.focus();
}
function choose(value: string) {
  emit('update:modelValue', value);
  close(true);
}
function scroll() {
  void nextTick(() =>
    root.value?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }),
  );
}
function key(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault();
    close(true);
  } else if (event.key === 'Tab') close();
  else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
    event.preventDefault();
    const last = filtered.value.length - 1;
    active.value =
      event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? last
          : Math.max(0, Math.min(last, active.value + (event.key === 'ArrowDown' ? 1 : -1)));
    scroll();
  } else if (event.key === 'Enter' || (!props.searchable && event.key === ' ')) {
    event.preventDefault();
    const option = filtered.value[active.value];
    if (option) choose(option.value);
  }
}
function outside(event: PointerEvent) {
  if (!root.value?.contains(event.target as Node)) close();
}
onMounted(() => document.addEventListener('pointerdown', outside));
onBeforeUnmount(() => document.removeEventListener('pointerdown', outside));
</script>
<template>
  <div ref="root" class="filter-select">
    <button
      ref="trigger"
      class="filter-trigger"
      :aria-label="label + ': ' + (chosen?.label || '')"
      :aria-expanded="open"
      :aria-haspopup="searchable ? 'dialog' : 'listbox'"
      :aria-controls="id"
      :disabled="disabled"
      @click="open ? close() : show()"
      @keydown.down.prevent="show"
      @keydown.up.prevent="show"
    >
      <span class="filter-leading"><AppIcon :name="icon || 'workflow'" :size="17" /></span
      ><span class="filter-trigger-text"
        ><small>{{ label }}</small
        ><strong>{{ chosen?.label || 'Select an option' }}</strong></span
      ><AppIcon name="down" :size="15" :class="{ rotated: open }" />
    </button>
    <div
      v-if="open"
      class="filter-popover"
      :role="searchable ? 'dialog' : undefined"
      :aria-label="searchable ? label : undefined"
      @keydown="key"
    >
      <div v-if="searchable" class="filter-search">
        <AppIcon name="search" :size="16" /><input
          ref="search"
          v-model="query"
          :placeholder="searchPlaceholder || 'Search…'"
          :aria-label="searchPlaceholder || 'Search options'"
          role="combobox"
          aria-expanded="true"
          :aria-controls="id"
          :aria-activedescendant="filtered[active] ? id + '-' + active : undefined"
          autocomplete="off"
          @input="active = 0"
        />
      </div>
      <div
        :id="id"
        ref="list"
        class="filter-options"
        role="listbox"
        :aria-label="label"
        tabindex="-1"
        :aria-activedescendant="filtered[active] ? id + '-' + active : undefined"
      >
        <div
          v-for="(option, i) in filtered"
          :id="id + '-' + i"
          :key="option.value"
          role="option"
          :aria-selected="modelValue === option.value"
          :data-active="active === i"
          class="filter-option"
          @pointermove="active = i"
          @click="choose(option.value)"
        >
          <span
            ><strong>{{ option.label }}</strong
            ><small v-if="option.description">{{ option.description }}</small></span
          ><AppIcon v-if="modelValue === option.value" name="check" :size="16" />
        </div>
        <p v-if="!filtered.length" class="filter-no-results">No matching journeys.</p>
      </div>
      <slot name="footer" />
    </div>
  </div>
</template>
