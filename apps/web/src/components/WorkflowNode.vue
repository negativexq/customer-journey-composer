<script setup lang="ts">
import { computed } from 'vue';
import { Handle, Position } from '@xyflow/vue';
import { ports, labels, type Kind, type Config } from '../lib/workflow';
import AppIcon from './ui/AppIcon.vue';
// NodeProps.draggable belongs to xyflow's wrapper, not native HTML drag-and-drop.
defineOptions({ inheritAttrs: false });
const props = defineProps<{
  data: {
    kind: Kind;
    label: string;
    config?: Config;
    invalid?: boolean;
    active?: boolean;
    visited?: boolean;
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
  email: 'email',
  sms: 'message',
  update_attribute: 'user',
  exit: 'exit',
};
function duration(seconds: number) {
  const [amount, unit] =
    seconds >= 86400
      ? [seconds / 86400, 'day']
      : seconds >= 3600
        ? [seconds / 3600, 'hour']
        : seconds >= 60
          ? [seconds / 60, 'minute']
          : [seconds, 'second'];
  return `${amount} ${unit}${amount === 1 ? '' : 's'}`;
}
const detail = computed(() => {
  const c = props.data.config;
  if (!c) return '';
  switch (props.data.kind) {
    case 'event_trigger':
      return c.event_name || 'Choose an event';
    case 'wait':
      return 'Wait ' + duration(c.duration_seconds || 0);
    case 'wait_event':
      return c.event_name || 'Choose an event';
    case 'email':
      return c.subject || 'Add email content';
    case 'sms':
      return c.text || 'Add message content';
    case 'condition':
      return c.rule?.path?.replace('customer.attributes.', '') || 'Configure conditions';
    case 'update_attribute':
      return c.attribute || 'Choose an attribute';
    case 'exit':
      return c.reason === 'converted' ? 'Conversion goal reached' : c.reason || 'Journey complete';
    default:
      return '';
  }
});
const label = (p: string) =>
  p === 'true'
    ? 'Yes'
    : p === 'false'
      ? 'No'
      : p === 'matched'
        ? 'Matched'
        : p === 'timeout'
          ? 'Timeout'
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
      },
    ]"
  >
    <Handle v-if="data.kind !== 'event_trigger'" id="in" type="target" :position="Position.Top" />
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
    <p class="workflow-node-detail">{{ detail }}</p>
    <div v-if="ports(data.kind).length > 1" class="node-ports">
      <span v-for="p in ports(data.kind)" :key="p" :class="p">{{ label(p) }}</span>
    </div>
    <div v-if="data.metrics" class="node-metrics">
      <span
        >Entered <strong>{{ data.metrics.entered }}</strong></span
      ><span
        >Waiting <strong>{{ data.metrics.waiting }}</strong></span
      ><span
        >Completed <strong>{{ data.metrics.completed }}</strong></span
      ><span
        >Failed <strong>{{ data.metrics.failed }}</strong></span
      ><small>Skipped {{ data.metrics.skipped }} · Avg {{ data.metrics.average }}</small>
    </div>
    <Handle
      v-for="(p, i) in ports(data.kind)"
      :id="p"
      :key="p"
      type="source"
      :position="Position.Bottom"
      :style="{ left: `${((i + 1) * 100) / (ports(data.kind).length + 1)}%` }"
    />
  </div>
</template>
