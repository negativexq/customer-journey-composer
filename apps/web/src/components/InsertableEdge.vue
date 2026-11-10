<script setup lang="ts">
import { computed } from 'vue';
import { BaseEdge, EdgeLabelRenderer, getBezierPath, type EdgeProps } from '@vue-flow/core';
import AppIcon from './ui/AppIcon.vue';
const props = defineProps<EdgeProps & { readonly?: boolean; connectionLabel?: string }>();
const emit = defineEmits<{ insert: [] }>();
const path = computed(() => getBezierPath(props));
</script>
<template>
  <BaseEdge
    :id="id"
    :path="path[0]"
    :marker-end="markerEnd"
    :label="label"
    :label-x="path[1]"
    :label-y="path[2] - 23"
  />
  <EdgeLabelRenderer v-if="!readonly">
    <button
      class="edge-insert nodrag nopan"
      :style="{ transform: `translate(-50%, -50%) translate(${path[1]}px, ${path[2]}px)` }"
      :aria-label="`Insert step · ${connectionLabel || label || 'Connection'}`"
      title="Insert a step on this connection"
      @click.stop="emit('insert')"
    >
      <AppIcon name="plus" :size="14" />
    </button>
  </EdgeLabelRenderer>
</template>
<style scoped>
.edge-insert {
  position: absolute;
  pointer-events: all;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  min-height: 30px;
  padding: 0;
  border: 1px solid #c9c2ef;
  border-radius: 50%;
  background: white;
  color: var(--accent);
  box-shadow: 0 1px 3px #20204010;
}
.edge-insert:hover,
.edge-insert:focus-visible {
  border-color: var(--accent);
  background: #f3f0ff;
}
</style>
