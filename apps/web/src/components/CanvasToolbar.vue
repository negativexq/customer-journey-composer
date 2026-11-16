<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';

// The floating bar under the canvas: panels, history, zoom and a menu of less used actions.
defineProps<{
  readonly?: boolean;
  aiOpen?: boolean;
  libraryOpen: boolean;
  zoomLevel: number;
  // Whether a step is selected, and whether its settings are showing.
  hasSelection: boolean;
  inspectorShown: boolean;
  locked: boolean;
  hasSharedExits: boolean;
}>();
const shortExitLinks = defineModel<boolean>('shortExitLinks', { required: true });
const emit = defineEmits<{
  'toggle-library': [];
  ai: [];
  undo: [];
  redo: [];
  'zoom-in': [];
  'zoom-out': [];
  fit: [];
  'toggle-inspector': [];
  layout: [];
  start: [];
  'focus-selected': [];
  'select-all': [];
  copy: [];
  paste: [];
  duplicate: [];
  'toggle-lock': [];
  shortcuts: [];
}>();
const menu = ref<HTMLDetailsElement>();
// Close the menu when the pointer goes anywhere else.
function closeOutside(event: PointerEvent) {
  if (menu.value?.open && !menu.value.contains(event.target as Node))
    menu.value.removeAttribute('open');
}
window.addEventListener('pointerdown', closeOutside);
onBeforeUnmount(() => window.removeEventListener('pointerdown', closeOutside));
function fromMenu(action: () => void) {
  menu.value?.removeAttribute('open');
  action();
}
</script>
<template>
  <div
    class="canvas-bar"
    :class="{ 'beside-ai': aiOpen }"
    role="toolbar"
    aria-label="Canvas controls"
  >
    <button
      type="button"
      class="bar-label"
      aria-label="Toggle step library"
      :aria-expanded="libraryOpen"
      @click="emit('toggle-library')"
    >
      <AppIcon name="templates" :size="16" /><span>Steps</span>
    </button>
    <button
      v-if="!readonly"
      type="button"
      class="bar-label bar-ai"
      aria-label="AI composer"
      :aria-pressed="!!aiOpen"
      @click="emit('ai')"
    >
      <AppIcon name="sparkles" :size="16" /><span>AI composer</span>
    </button>
    <i aria-hidden="true" />
    <button
      v-for="[title, icon, handler] in [
        ['Undo', 'undo', () => emit('undo')],
        ['Redo', 'redo', () => emit('redo')],
      ] as const"
      :key="title"
      type="button"
      :disabled="readonly"
      :aria-label="title"
      :title="title"
      @click="handler"
    >
      <AppIcon :name="icon" :size="18" />
    </button>
    <i aria-hidden="true" />
    <button
      type="button"
      class="bar-zoom"
      aria-label="Zoom out"
      title="Zoom out"
      @click="emit('zoom-out')"
    >
      <AppIcon name="minus" :size="18" /></button
    ><span class="canvas-zoom-level bar-zoom" aria-label="Zoom level">{{ zoomLevel }}%</span
    ><button
      type="button"
      class="bar-zoom"
      aria-label="Zoom in"
      title="Zoom in"
      @click="emit('zoom-in')"
    >
      <AppIcon name="plus" :size="18" /></button
    ><button type="button" aria-label="Fit view" title="Fit view" @click="emit('fit')">
      <AppIcon name="maximize" :size="18" />
    </button>
    <i aria-hidden="true" />
    <button
      type="button"
      aria-label="Toggle step settings"
      title="Step settings"
      :aria-expanded="inspectorShown"
      :disabled="!hasSelection"
      @click="emit('toggle-inspector')"
    >
      <AppIcon name="settings" :size="18" />
    </button>
    <details ref="menu" class="bar-more">
      <summary aria-label="More canvas actions" title="More">
        <AppIcon name="more" :size="18" />
      </summary>
      <div>
        <label
          v-if="hasSharedExits"
          class="menu-check"
          title="Shared exits appear as references. Select one to open the original step."
          ><input v-model="shortExitLinks" type="checkbox" />Short exit links</label
        >
        <button type="button" :disabled="readonly" @click="fromMenu(() => emit('layout'))">
          Auto layout
        </button>
        <button type="button" @click="fromMenu(() => emit('start'))">Go to start</button>
        <button
          type="button"
          :disabled="!hasSelection"
          @click="fromMenu(() => emit('focus-selected'))"
        >
          Focus selected step
        </button>
        <hr />
        <button type="button" :disabled="readonly" @click="fromMenu(() => emit('select-all'))">
          Select all nodes
        </button>
        <button type="button" :disabled="readonly" @click="fromMenu(() => emit('copy'))">
          Copy selection
        </button>
        <button type="button" :disabled="readonly" @click="fromMenu(() => emit('paste'))">
          Paste selection
        </button>
        <button type="button" :disabled="readonly" @click="fromMenu(() => emit('duplicate'))">
          Duplicate node
        </button>
        <hr />
        <button v-if="!readonly" type="button" @click="fromMenu(() => emit('toggle-lock'))">
          {{ locked ? 'Unlock canvas' : 'Lock canvas' }}
        </button>
        <button type="button" @click="fromMenu(() => emit('shortcuts'))">Keyboard shortcuts</button>
      </div>
    </details>
  </div>
</template>
