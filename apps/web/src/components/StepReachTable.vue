<script setup lang="ts">
import { computed } from 'vue';
import { metric, type Insights } from '../lib/insights';
import type { Workflow } from '../lib/workflow';

// How many customers reached, completed and wait at each step, in workflow order. A step name
// opens the step; a count opens the executions behind it.
const props = defineProps<{ workflow: Workflow; insights: Insights }>();
const emit = defineEmits<{ select: [string]; people: [id: string, waiting: boolean] }>();
const value = (dimension: string, name: string, key: string) =>
  metric(props.insights, dimension, name, key);
const graphOrder = computed(() => {
  const w = props.workflow;
  const indegree = new Map(w.nodes.map((n) => [n.id, 0]));
  for (const e of w.edges) indegree.set(e.target, (indegree.get(e.target) || 0) + 1);
  const queue = w.nodes.filter((n) => !indegree.get(n.id));
  const result: Workflow['nodes'] = [];
  while (queue.length) {
    const n = queue.shift()!;
    result.push(n);
    for (const e of w.edges.filter((e) => e.source === n.id)) {
      indegree.set(e.target, indegree.get(e.target)! - 1);
      if (!indegree.get(e.target)) {
        const next = w.nodes.find((n) => n.id === e.target);
        if (next) queue.push(next);
      }
    }
  }
  return result;
});
</script>
<template>
  <section class="surface">
    <div class="surface-heading">
      <div>
        <h3>Step reach</h3>
        <p>In workflow order. Customers on another branch did not drop off.</p>
      </div>
    </div>
    <div class="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Step</th>
            <th>Reached</th>
            <th>Completed step</th>
            <th>Waiting now</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="n in graphOrder" :key="n.id">
            <td>
              <button class="journey-name-button" @click="emit('select', n.id)">
                {{ n.label }}
              </button>
            </td>
            <td>
              <button
                v-if="value('node', 'entered', n.id)"
                class="metric-link"
                :aria-label="`Show ${value('node', 'entered', n.id)} executions that reached ${n.label}`"
                @click="emit('people', n.id, false)"
              >
                {{ value('node', 'entered', n.id) }}</button
              ><template v-else>0</template>
            </td>
            <td>{{ value('node', 'SUCCEEDED', n.id) }}</td>
            <td>
              <button
                v-if="value('node_current', 'WAITING', n.id)"
                class="metric-link"
                :aria-label="`Show ${value('node_current', 'WAITING', n.id)} executions waiting at ${n.label}`"
                @click="emit('people', n.id, true)"
              >
                {{ value('node_current', 'WAITING', n.id) }}</button
              ><template v-else>0</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
