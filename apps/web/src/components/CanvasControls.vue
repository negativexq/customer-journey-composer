<script setup lang="ts">
import { computed } from 'vue';
import { Controls, useVueFlow } from '@xyflow/vue';
import AppIcon from './ui/AppIcon.vue';
defineProps<{ interactive?: boolean }>();
const { viewport } = useVueFlow();
const zoom = computed(() => Math.round(viewport.value.zoom * 100));
</script>
<template>
  <Controls :show-interactive="!!interactive" aria-label="Canvas controls">
    <template #icon-zoom-in><AppIcon name="plus" :size="18" /></template>
    <template #icon-zoom-out><AppIcon name="minus" :size="18" /></template>
    <template #icon-fit-view><AppIcon name="maximize" :size="18" /></template>
    <template #icon-lock><AppIcon name="lock" :size="18" /></template>
    <template #icon-unlock><AppIcon name="unlock" :size="18" /></template>
    <div class="canvas-zoom-level" aria-label="Zoom level">{{ zoom }}%</div>
  </Controls>
</template>
