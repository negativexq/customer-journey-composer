<script setup lang="ts">
import { computed, inject } from 'vue';
import { Handle, Position } from '@xyflow/vue';
import { ports, labels, type Kind, type Config } from '../lib/workflow';
import AppIcon from './ui/AppIcon.vue';
import { stepSummary } from '../lib/workflow-presentation';
import type { StepCaption } from '../lib/step-captions';
// NodeProps.draggable belongs to xyflow's wrapper, not native HTML drag-and-drop.
defineOptions({ inheritAttrs: false });
const props = defineProps<{
  id?: string;
  data: {
    kind: Kind;
    label: string;
    config?: Config;
    invalid?: boolean;
    active?: boolean;
    visited?: boolean;
    muted?: boolean;
    // When a customer arrives here; shown as the card's first line.
    caption?: StepCaption;
    // Outputs with nothing connected yet. Each gets an add button in the builder.
    openPorts?: string[];
    observations?: { label: string; value: number }[];
    metrics?: {
      entered: number;
      waiting: number;
      completed: number;
      failed: number;
      skipped: number;
      average: string;
    };
  };
  selected?: boolean;
}>();
const icons: Record<string, string> = {
  event_trigger: 'zap',
  wait: 'clock',
  wait_event: 'events',
  condition: 'branch',
  switch: 'branch',
  split: 'split',
  email: 'email',
  sms: 'message',
  update_attribute: 'user',
  exit: 'exit',
};
// Provided by the builder canvas only; read-only canvases have no add buttons.
const addAfter = inject<((nodeID: string, port: string) => void) | undefined>(
  'canvas-add-step',
  undefined,
);
const portPosition = (port: string) => {
  const all = ports(props.data.kind, props.data.config);
  return `${((all.indexOf(port) + 1) * 100) / (all.length + 1)}%`;
};
const detail = computed(() =>
  stepSummary({ type: props.data.kind, config: props.data.config || {} }),
);
const label = (p: string) =>
  p === 'true'
    ? 'Yes'
    : p === 'false'
      ? 'No'
      : p === 'matched'
        ? 'Matched'
        : p === 'timeout'
          ? 'Timeout'
          : p === 'else'
            ? 'Otherwise'
            : p.startsWith('case_')
              ? props.data.config?.branches?.[Number(p.slice(5)) - 1]?.label ||
                'Branch ' + p.slice(5)
              : p === 'a'
                ? 'A'
                : p === 'b'
                  ? 'B'
                  : '';
</script>
<template>
  <div
    class="workflow-node"
    :class="[
      data.kind,
      {
        chosen: selected,
        invalid: data.invalid,
        'is-current': data.active,
        'was-visited': data.visited,
        'is-muted': data.muted,
      },
    ]"
  >
    <Handle v-if="data.kind !== 'event_trigger'" id="in" type="target" :position="Position.Top" />
    <p v-if="data.caption" class="node-caption" :class="data.caption.tone">
      {{ data.caption.text }}
    </p>
    <div class="workflow-node-heading">
      <span class="step-type-icon" :class="data.kind"
        ><AppIcon :name="icons[data.kind]" :size="18"
      /></span>
      <div>
        <small>{{ labels[data.kind] }}</small
        ><strong class="node-title">{{ data.label }}</strong>
      </div>
      <span v-if="data.invalid" class="node-warning" title="Needs attention">!</span>
    </div>
    <p class="workflow-node-detail" :title="detail">{{ detail }}</p>
    <div v-if="data.metrics" class="node-metrics">
      <template v-if="data.observations"
        ><span v-for="item in data.observations" :key="item.label"
          >{{ item.label }} <strong>{{ item.value }}</strong></span
        ></template
      ><template v-else
        ><span
          >Entered <strong>{{ data.metrics.entered }}</strong></span
        ><span
          >Waiting <strong>{{ data.metrics.waiting }}</strong></span
        ><span
          >Completed <strong>{{ data.metrics.completed }}</strong></span
        ><span
          >Failed <strong>{{ data.metrics.failed }}</strong></span
        ><small
          >Skipped {{ data.metrics.skipped }} · Avg {{ data.metrics.average }}</small
        ></template
      >
    </div>
    <Handle
      v-for="p in ports(data.kind, data.config)"
      :id="p"
      :key="p"
      type="source"
      :position="Position.Bottom"
      :style="{ left: portPosition(p) }"
    />
    <template v-if="addAfter && id">
      <button
        v-for="p in data.openPorts || []"
        :key="'add-' + p"
        type="button"
        class="node-add nodrag nopan"
        :style="{ left: portPosition(p) }"
        :aria-label="`Add a step after ${data.label}${label(p) ? ' · ' + label(p) : ''}`"
        :title="label(p) ? `Add a step on ${label(p)}` : 'Add the next step'"
        @click.stop="addAfter(id, p)"
      >
        <AppIcon name="plus" :size="14" />
      </button>
    </template>
  </div>
</template>
