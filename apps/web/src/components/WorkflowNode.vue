<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core';
import { ports, labels, type Kind } from '../lib/workflow';
defineProps<{ data: { kind: Kind; label: string; invalid?: boolean }; selected?: boolean }>();
</script>
<template>
  <div class="workflow-node" :class="[data.kind, { chosen: selected, invalid: data.invalid }]">
    <Handle v-if="data.kind !== 'event_trigger'" type="target" :position="Position.Top" /><small>{{
      labels[data.kind]
    }}</small
    ><strong>{{ data.label }}</strong
    ><span v-if="data.invalid" class="error-message">Needs attention</span>
    <div class="node-ports">
      <span v-for="port in ports(data.kind)" :key="port">{{ port }}</span>
    </div>
    <Handle
      v-for="(port, index) in ports(data.kind)"
      :id="port"
      :key="port"
      type="source"
      :position="Position.Bottom"
      :style="{ left: `${((index + 1) * 100) / (ports(data.kind).length + 1)}%` }"
    />
  </div>
</template>
