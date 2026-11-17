<script setup lang="ts">
import { DropdownMenuCheckboxItem, DropdownMenuItem, DropdownMenuSeparator } from 'reka-ui';
import AppIcon from './ui/AppIcon.vue';
import OverflowMenu from './ui/OverflowMenu.vue';

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
    <OverflowMenu label="More canvas actions" side="top" :icon-size="18" trigger-class="bar-more">
      <DropdownMenuCheckboxItem
        v-if="hasSharedExits"
        v-model="shortExitLinks"
        class="overflow-item overflow-check"
        title="Shared exits appear as references. Select one to open the original step."
        @select.prevent
        ><span class="overflow-box" aria-hidden="true"
          ><AppIcon v-if="shortExitLinks" name="check" :size="12" /></span
        >Short exit links</DropdownMenuCheckboxItem
      >
      <DropdownMenuItem class="overflow-item" :disabled="readonly" @select="emit('layout')"
        >Auto layout</DropdownMenuItem
      >
      <DropdownMenuItem class="overflow-item" @select="emit('start')">Go to start</DropdownMenuItem>
      <DropdownMenuItem
        class="overflow-item"
        :disabled="!hasSelection"
        @select="emit('focus-selected')"
        >Focus selected step</DropdownMenuItem
      >
      <DropdownMenuSeparator class="overflow-separator" />
      <DropdownMenuItem class="overflow-item" :disabled="readonly" @select="emit('select-all')"
        >Select all nodes</DropdownMenuItem
      >
      <DropdownMenuItem class="overflow-item" :disabled="readonly" @select="emit('copy')"
        >Copy selection</DropdownMenuItem
      >
      <DropdownMenuItem class="overflow-item" :disabled="readonly" @select="emit('paste')"
        >Paste selection</DropdownMenuItem
      >
      <DropdownMenuItem class="overflow-item" :disabled="readonly" @select="emit('duplicate')"
        >Duplicate node</DropdownMenuItem
      >
      <DropdownMenuSeparator class="overflow-separator" />
      <DropdownMenuItem v-if="!readonly" class="overflow-item" @select="emit('toggle-lock')">{{
        locked ? 'Unlock canvas' : 'Lock canvas'
      }}</DropdownMenuItem>
      <DropdownMenuItem class="overflow-item" @select="emit('shortcuts')"
        >Keyboard shortcuts</DropdownMenuItem
      >
    </OverflowMenu>
  </div>
</template>
