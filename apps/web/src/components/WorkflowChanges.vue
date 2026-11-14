<script setup lang="ts">
import { labels } from '../lib/workflow';
import { stepSummary } from '../lib/workflow-presentation';
import type { WorkflowDiff } from '../lib/version-diff';
defineProps<{ diff: WorkflowDiff }>();
</script>
<template>
  <ul class="workflow-changes">
    <li v-for="step in diff.added" :key="'a' + step.id">
      <span class="change-kind added">Added</span>
      <span class="change-copy"
        ><strong>{{ step.label }}</strong
        ><small>{{ labels[step.type] }} · {{ stepSummary(step) }}</small></span
      >
    </li>
    <li v-for="step in diff.removed" :key="'r' + step.id">
      <span class="change-kind removed">Removed</span>
      <span class="change-copy"
        ><strong>{{ step.label }}</strong
        ><small>{{ labels[step.type] }} · {{ stepSummary(step) }}</small></span
      >
    </li>
    <li v-for="change in diff.changed" :key="'c' + change.step.id">
      <span class="change-kind changed">Changed</span>
      <span class="change-copy"
        ><strong>{{ change.step.label }}</strong
        ><small v-for="line in change.changes" :key="line">{{ line }}</small></span
      >
    </li>
    <li v-if="diff.connectionsChanged">
      <span class="change-kind changed">Changed</span>
      <span class="change-copy"
        ><strong>Connections</strong><small>Steps are routed differently.</small></span
      >
    </li>
    <li v-if="diff.exitChanged">
      <span class="change-kind changed">Changed</span>
      <span class="change-copy"
        ><strong>Global exit</strong
        ><small>The event or rule that ends the journey early is different.</small></span
      >
    </li>
    <li v-if="diff.reentryChanged">
      <span class="change-kind changed">Changed</span>
      <span class="change-copy"
        ><strong>Re-entry</strong
        ><small>The rule for entering the journey again is different.</small></span
      >
    </li>
  </ul>
</template>
<style scoped>
.workflow-changes {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
  display: grid;
  gap: 8px;
}
li {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}
.change-kind {
  justify-self: start;
  padding: 3px 7px;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 600;
}
.added {
  background: #e8f6ee;
  color: #1c6b43;
}
.removed {
  background: #fdecea;
  color: #a92c24;
}
.changed {
  background: #fff4e0;
  color: #8a5a00;
}
.change-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.change-copy strong {
  font-weight: 600;
  font-size: 13px;
  color: var(--text);
}
.change-copy small {
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
</style>
