<script setup lang="ts">
import { computed, ref } from 'vue';
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath, type EdgeProps } from '@xyflow/vue';
import AppIcon from './ui/AppIcon.vue';
const props = defineProps<
  EdgeProps & {
    readonly?: boolean;
    connectionLabel?: string;
    highlighted?: boolean;
    muted?: boolean;
    exitReference?: string;
  }
>();
const emit = defineEmits<{ insert: []; jump: [] }>();
const hovered = ref(false);
const direction = computed(() => (['No', 'Timeout', 'B'].includes(String(props.label)) ? 1 : -1));
const referenceX = computed(() => props.sourceX + direction.value * 170);
const referenceY = computed(() => props.sourceY + 18);
const path = computed(() =>
  props.exitReference
    ? ([
        `M ${props.sourceX},${props.sourceY} V ${referenceY.value} H ${props.sourceX + direction.value * 86}`,
        referenceX.value,
        referenceY.value,
      ] as [string, number, number])
    : getSmoothStepPath({ ...props, offset: 38, borderRadius: 10 }),
);
</script>
<template>
  <g @mouseenter="hovered = true" @mouseleave="hovered = false"
    ><BaseEdge
      :id="id"
      :path="path[0]"
      :marker-end="markerEnd"
      :style="{
        stroke: highlighted ? 'var(--edge-active)' : muted ? 'var(--edge-muted)' : 'var(--edge)',
        strokeWidth: highlighted ? 2.4 : 1.5,
        opacity: muted ? 0.45 : 1,
      }"
  /></g>
  <EdgeLabelRenderer>
    <span
      v-if="label && !exitReference"
      class="connection-port-label"
      :class="{ muted }"
      :style="{ transform: `translate(-50%, 0) translate(${sourceX}px, ${sourceY + 9}px)` }"
      >{{ label }}</span
    >
    <div
      v-if="exitReference"
      class="exit-reference nodrag nopan"
      :class="{ muted }"
      :style="{ transform: `translate(-50%, -50%) translate(${referenceX}px, ${referenceY}px)` }"
    >
      <button
        type="button"
        :title="`Open shared exit: ${exitReference}`"
        :aria-label="`Go to shared exit · ${connectionLabel}`"
        @click.stop="emit('jump')"
      >
        <small>{{ label }} · shared exit</small><strong>{{ exitReference }} ↗</strong></button
      ><button
        v-if="!readonly"
        type="button"
        class="exit-reference-insert"
        :aria-label="`Insert step · ${connectionLabel}`"
        title="Insert a step before this exit"
        @click.stop="emit('insert')"
      >
        <AppIcon name="plus" :size="12" />
      </button>
    </div>
    <div
      v-else
      class="edge-actions nodrag nopan"
      :class="{ revealed: hovered || highlighted }"
      :style="{ transform: `translate(-50%, -50%) translate(${path[1]}px, ${path[2]}px)` }"
    >
      <button
        v-if="!readonly"
        class="edge-insert"
        type="button"
        :aria-label="`Insert step · ${connectionLabel || label || 'Connection'}`"
        title="Insert a step on this connection"
        @click.stop="emit('insert')"
      >
        <AppIcon name="plus" :size="14" />
      </button>
      <button
        class="edge-jump"
        type="button"
        :aria-label="`Go to target · ${connectionLabel || 'Connection'}`"
        title="Go to connected step"
        @click.stop="emit('jump')"
      >
        <AppIcon name="arrow" :size="14" />
      </button>
    </div>
  </EdgeLabelRenderer>
</template>
<style scoped>
.edge-actions {
  position: absolute;
  pointer-events: all;
  display: flex;
  gap: 3px;
  opacity: 0;
  transition: opacity 0.12s;
}
.edge-actions.revealed,
.edge-actions:hover,
.edge-actions:focus-within {
  opacity: 1;
}
.edge-actions button {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  min-height: 28px;
  padding: 0;
  border: 1px solid var(--violet-85);
  border-radius: 6px;
  background: var(--surface);
  color: var(--accent-text);
  box-shadow: 0 1px 3px #20204010;
}
.edge-actions button:hover {
  border-color: var(--accent);
  background: var(--violet-96);
}
.connection-port-label {
  position: absolute;
  font-size: 10px;
  line-height: 1.4;
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--surface);
  color: var(--gray-52);
  pointer-events: none;
}
.connection-port-label.muted {
  opacity: 0.45;
}
@media (prefers-reduced-motion: reduce) {
  .edge-actions {
    transition: none;
  }
}

.exit-reference {
  position: absolute;
  width: 168px;
  height: 34px;
  display: flex;
  pointer-events: all;
  border: 1px dashed var(--violet-79);
  border-radius: 6px;
  background: var(--gray-99);
}
.exit-reference.muted {
  opacity: 0.45;
}
.exit-reference > button:first-child {
  display: grid;
  justify-content: stretch;
  justify-items: start;
  gap: 1px;
  min-width: 0;
  flex: 1;
  padding: 3px 7px;
  border: 0;
  background: transparent;
  text-align: left;
  min-height: 0;
}
.exit-reference small {
  font-size: 9px;
  color: var(--violet-58);
  line-height: 1.2;
}
.exit-reference strong {
  font-size: 10px;
  font-weight: 500;
  line-height: 1.2;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
  width: 100%;
  color: var(--violet-47b);
}
.exit-reference .exit-reference-insert {
  padding: 0 4px;
  min-height: 0;
  border: 0;
  border-left: 1px solid var(--violet-92);
  background: transparent;
  opacity: 0;
}
.exit-reference:hover .exit-reference-insert,
.exit-reference:focus-within .exit-reference-insert {
  opacity: 1;
}
</style>
