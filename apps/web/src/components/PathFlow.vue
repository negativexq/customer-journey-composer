<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import EmptyState from './ui/EmptyState.vue';
import { metric, type Insights } from '../lib/insights';
import { pathFlow, type FlowLink, type FlowNode } from '../lib/path-flow';
import { plural } from '../lib/format';
import type { Workflow } from '../lib/workflow';
import { portLabel } from '../lib/workflow-presentation';

// Where customers actually went: a bar per step and a band per connection, both sized by
// customers. One hue carries magnitude; the step table below holds the same numbers as text.
const props = defineProps<{ workflow: Workflow; insights: Insights }>();
const emit = defineEmits<{ branch: [{ node: string; port: string }]; step: [string] }>();
const frame = ref<HTMLDivElement>();
const available = ref(960);
const height = 300;
const barWidth = 10;
let observer: ResizeObserver | undefined;
onMounted(() => {
  if (!frame.value) return;
  observer = new ResizeObserver(([entry]) => (available.value = entry!.contentRect.width));
  observer.observe(frame.value);
});
onBeforeUnmount(() => observer?.disconnect());

const columns = computed(
  () =>
    pathFlow(props.workflow, counts.value, { width: 1000 }).nodes.reduce(
      (max, node) => Math.max(max, node.column),
      0,
    ) + 1,
);
const counts = computed(() => ({
  reached: (id: string) => metric(props.insights, 'node', 'entered', id),
  taken: (id: string, port: string) => metric(props.insights, 'edge', 'traversed', id + ':' + port),
}));
// Every column needs room for its label; a wide journey scrolls sideways instead of squeezing.
const width = computed(() => Math.max(available.value, columns.value * 150));
const labelRoom = 150;
const flow = computed(() =>
  pathFlow(props.workflow, counts.value, { width: width.value - labelRoom, height, barWidth }),
);
const names = computed(() => new Map(props.workflow.nodes.map((node) => [node.id, node.label])));
const branching = (id: string) =>
  props.workflow.edges.filter((edge) => edge.source === id).length > 1;
const percent = (share: number) => Math.round(share * 100) + '%';
const linkText = (link: FlowLink) =>
  `${names.value.get(link.source)} → ${names.value.get(link.target)}${
    branching(link.source) ? ' · ' + portLabel(link.port) : ''
  }: ${plural(link.value, 'customer')} (${percent(link.share)} of the step)`;
const nodeText = (node: FlowNode) =>
  `${node.label}: ${plural(node.value, 'customer')} (${percent(flow.value.total ? node.value / flow.value.total : 0)} of entries)`;

const hovered = ref('');
const tip = ref<{ text: string; x: number; y: number }>();
function show(event: MouseEvent, key: string, text: string) {
  const box = frame.value!.getBoundingClientRect();
  hovered.value = key;
  tip.value = {
    text,
    x: event.clientX - box.left + frame.value!.scrollLeft,
    y: event.clientY - box.top,
  };
}
function hide() {
  hovered.value = '';
  tip.value = undefined;
}
</script>
<template>
  <section class="surface path-flow" aria-label="Customer paths">
    <div class="surface-heading">
      <div>
        <h3>Customer paths</h3>
        <p>
          Bar height is customers who reached a step; band width is customers who took that
          connection. Select a band to list them.
        </p>
      </div>
    </div>
    <EmptyState v-if="!flow.nodes.length" icon="branch" title="No paths to draw yet" compact>
      Paths appear once customers enter this version in the selected period.
    </EmptyState>
    <div v-else ref="frame" class="path-flow-frame" @mouseleave="hide">
      <svg
        :width="width"
        :height="height + 28"
        :viewBox="`0 -14 ${width} ${height + 28}`"
        role="group"
        aria-label="Flow of customers between steps"
      >
        <path
          v-for="link in flow.links"
          :key="link.id"
          class="flow-band"
          :class="{ active: hovered === 'l' + link.id, dim: hovered && hovered !== 'l' + link.id }"
          :d="link.path"
          role="button"
          tabindex="0"
          :aria-label="'Show customers. ' + linkText(link)"
          @mousemove="show($event, 'l' + link.id, linkText(link))"
          @focus="hovered = 'l' + link.id"
          @blur="hide"
          @click="emit('branch', { node: link.source, port: link.port })"
          @keydown.enter.prevent="emit('branch', { node: link.source, port: link.port })"
        />
        <g
          v-for="node in flow.nodes"
          :key="node.id"
          class="flow-step"
          role="button"
          tabindex="0"
          :aria-label="'Show customers. ' + nodeText(node)"
          @mousemove="show($event, 'n' + node.id, nodeText(node))"
          @focus="hovered = 'n' + node.id"
          @blur="hide"
          @click="emit('step', node.id)"
          @keydown.enter.prevent="emit('step', node.id)"
        >
          <!-- A wider transparent target than the 10px bar. -->
          <rect
            class="flow-hit"
            :x="node.x - 6"
            :y="node.y - 4"
            :width="barWidth + 12"
            :height="node.height + 8"
          />
          <rect
            class="flow-bar"
            :x="node.x"
            :y="node.y"
            :width="barWidth"
            :height="node.height"
            rx="3"
          />
          <text :x="node.x + barWidth + 8" :y="node.y + 11" class="flow-label">
            {{ node.label }}
          </text>
          <text :x="node.x + barWidth + 8" :y="node.y + 26" class="flow-value">
            {{ node.value }} · {{ percent(flow.total ? node.value / flow.total : 0) }}
          </text>
        </g>
      </svg>
      <p
        v-if="tip"
        class="flow-tip"
        role="status"
        :style="{ left: tip.x + 'px', top: tip.y + 'px' }"
      >
        {{ tip.text }}
      </p>
    </div>
  </section>
</template>
<style scoped>
.path-flow {
  margin: 16px 0;
}
.path-flow-frame {
  position: relative;
  padding: 8px 18px 14px;
  overflow-x: auto;
}
svg {
  display: block;
}
.flow-band {
  fill: var(--accent);
  fill-opacity: 0.2;
  cursor: pointer;
  transition: fill-opacity 0.14s ease-out;
}
.flow-band.active,
.flow-band:focus-visible {
  fill-opacity: 0.5;
}
.flow-band.dim {
  fill-opacity: 0.1;
}
.flow-step {
  cursor: pointer;
}
.flow-hit {
  fill: transparent;
}
.flow-bar {
  fill: var(--accent);
}
.flow-band:focus-visible,
.flow-step:focus-visible {
  outline: 2px solid var(--accent-text);
  outline-offset: 2px;
}
.flow-label {
  fill: var(--text);
  font-size: 12px;
  font-weight: 600;
}
.flow-value {
  fill: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.flow-tip {
  position: absolute;
  z-index: 3;
  max-width: 280px;
  margin: 0;
  padding: 6px 9px;
  transform: translate(12px, 12px);
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font-size: 12px;
  line-height: 1.4;
  box-shadow: 0 8px 24px #17213b24;
  pointer-events: none;
}
@media (prefers-reduced-motion: reduce) {
  .flow-band {
    transition: none;
  }
}
</style>
