<script setup lang="ts">
import {
  DropdownMenuContent,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui';
import AppIcon from './AppIcon.vue';

// A "more actions" menu. Reka handles outside presses (including ones other handlers
// swallow), focus, arrow keys and Escape, so it never stays open by accident.
// Put DropdownMenuItem / DropdownMenuSeparator children in the default slot.
withDefaults(
  defineProps<{
    label: string;
    side?: 'top' | 'bottom';
    iconSize?: number;
    triggerClass?: string;
  }>(),
  { side: 'bottom', iconSize: 19, triggerClass: '' },
);
</script>
<template>
  <DropdownMenuRoot :modal="false">
    <DropdownMenuTrigger
      class="overflow-trigger"
      :class="triggerClass"
      :aria-label="label"
      :title="label"
    >
      <AppIcon name="more" :size="iconSize" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        class="overflow-menu"
        :side="side"
        align="end"
        :side-offset="6"
        :collision-padding="8"
      >
        <slot />
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
