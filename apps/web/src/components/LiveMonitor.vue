<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { VueFlow, type Node, type Edge } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { MiniMap } from '@vue-flow/minimap';
import { api } from '../lib/api';
import type { Workflow } from '../lib/workflow';
const props = defineProps<{ journeyId: string; readonly?: boolean; admin?: boolean }>();
type Execution = {
  id: string;
  version_id: string;
  customer_id: string;
  state: string;
  current_node_id: string;
  started_at: string;
  exit_reason?: string;
  steps: {
    node_id: string;
    state: string;
    output: { port?: string };
    started_at: string;
    completed_at?: string;
  }[];
  jobs: unknown[];
  waiters: unknown[];
  deliveries: {
    id: string;
    node_id: string;
    channel: string;
    status: string;
    decision: unknown;
    attempts: number;
    last_error?: string;
  }[];
};
const versions = ref<{ id: string; version: number }[]>([]);
const version = ref(1);
const workflow = ref<Workflow>();
const metrics = ref<{ node_id: string; metric: string; value: number }[]>([]);
const customer = ref('');
const filter = ref('');
const executions = ref<Execution[]>([]);
const selected = ref<Execution>();
const error = ref('');
const busy = ref(false);
const poll = ref(true);
const delivery = ref<unknown>();
const stamp = ref('');
let timer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;
const value = (node: string, metric: string) =>
  metrics.value.find((m) => m.node_id === node && m.metric === metric)?.value || 0;
const nodes = computed<Node[]>(
  () =>
    workflow.value?.nodes.map((n, i) => ({
      id: n.id,
      position: n.position || { x: (i % 3) * 270, y: Math.floor(i / 3) * 200 },
      data: {
        label:
          n.label +
          '\nEntered ' +
          value(n.id, 'entered') +
          ' · Waiting ' +
          value(n.id, 'WAITING') +
          '\nCompleted ' +
          value(n.id, 'SUCCEEDED') +
          ' · Skipped ' +
          value(n.id, 'SKIPPED') +
          ' · Failed ' +
          value(n.id, 'FAILED') +
          '\nAvg ' +
          (value(n.id, 'duration_ms') / Math.max(1, value(n.id, 'finished_count')) / 1000).toFixed(
            1,
          ) +
          's',
      },
      style: {
        whiteSpace: 'pre-line',
        width: '240px',
        border:
          selected.value?.current_node_id === n.id
            ? '3px solid #3c6634'
            : selected.value?.steps?.some((s) => s.node_id === n.id)
              ? '3px solid #b09a5c'
              : '1px solid #ccd7c5',
        background: selected.value?.current_node_id === n.id ? '#edf4e7' : 'white',
      },
    })) || [],
);
const edges = computed<Edge[]>(
  () =>
    workflow.value?.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      label: e.port,
      animated: selected.value?.steps?.some(
        (s) => s.node_id === e.source && s.output?.port === e.port,
      ),
      style: {
        stroke: selected.value?.steps?.some(
          (s) => s.node_id === e.source && s.output?.port === e.port,
        )
          ? '#3c6634'
          : '#b9c7b4',
      },
    })) || [],
);
async function graph() {
  workflow.value = (
    await api<{ workflow: Workflow }>(`/journeys/${props.journeyId}/versions/${version.value}`)
  ).workflow;
}
async function refresh() {
  if (busy.value || disposed) return;
  busy.value = true;
  error.value = '';
  try {
    if (!versions.value.length) {
      versions.value = await api(`/journeys/${props.journeyId}/versions`);
      if (versions.value.length) {
        version.value = versions.value[0]!.version;
        await graph();
      }
    }
    const params = new URLSearchParams();
    if (customer.value) params.set('customer_id', customer.value);
    if (filter.value) params.set('state', filter.value);
    [executions.value, metrics.value] = await Promise.all([
      api<Execution[]>(`/journeys/${props.journeyId}/executions?${params}`),
      api<typeof metrics.value>(`/journeys/${props.journeyId}/node-stats?version=${version.value}`),
    ]);
    if (selected.value) selected.value = await api<Execution>('/executions/' + selected.value.id);
    stamp.value = new Date().toLocaleTimeString();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
    if (!disposed) {
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (poll.value && !document.hidden) void refresh();
        else schedule();
      }, 5000);
    }
  }
}
function schedule() {
  if (disposed) return;
  clearTimeout(timer);
  timer = setTimeout(() => {
    if (poll.value && !document.hidden) void refresh();
    else schedule();
  }, 5000);
}
async function track(id: string) {
  try {
    selected.value = await api<Execution>('/executions/' + id);
    const v = versions.value.find((v) => v.id === selected.value?.version_id);
    if (v && v.version !== version.value) {
      version.value = v.version;
      await graph();
    }
    await refresh();
  } catch (cause) {
    error.value = String(cause);
  }
}
async function control(action: string) {
  if (!selected.value) return;
  try {
    await api(`/executions/${selected.value.id}/${action}`, 'POST');
    await refresh();
  } catch (cause) {
    error.value = String(cause);
  }
}
async function inspect(id: string) {
  try {
    delivery.value = await api('/deliveries/' + id);
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(() => void refresh());
onBeforeUnmount(() => {
  disposed = true;
  clearTimeout(timer);
});
</script>
<template>
  <section>
    <div class="page-heading">
      <div>
        <h2>Live Monitor</h2>
        <p>
          Refresh every 5s while visible · {{ stamp }}. Gold: visited path; green: current node.
          Counts are asynchronously projected.
        </p>
      </div>
      <button :disabled="busy" @click="refresh">Refresh monitor</button>
    </div>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <div class="editor-form">
      <label>Track customer ID<input v-model="customer" @keyup.enter="refresh" /></label
      ><button @click="refresh">Track customer</button
      ><label
        >Execution state<select v-model="filter" @change="refresh">
          <option value="">All</option>
          <option
            v-for="s in ['RUNNING', 'WAITING', 'COMPLETED', 'EXITED', 'FAILED', 'CANCELLED']"
            :key="s"
          >
            {{ s }}
          </option>
        </select></label
      ><label
        >Published version<select
          v-model.number="version"
          @change="
            selected = undefined;
            graph();
            refresh();
          "
        >
          <option v-for="v in versions" :key="v.id" :value="v.version">v{{ v.version }}</option>
        </select></label
      ><label><input v-model="poll" type="checkbox" /> Auto refresh</label>
    </div>
    <p v-if="!versions.length" class="panel">
      Publish a validated journey to monitor live executions.
    </p>
    <div v-if="workflow" class="monitor-canvas">
      <VueFlow
        id="monitor"
        :nodes="nodes"
        :edges="edges"
        :nodes-draggable="false"
        :nodes-connectable="false"
        fit-view-on-init
        ><Background /><Controls /><MiniMap
      /></VueFlow>
    </div>
    <section class="panel">
      <h3>Executions</h3>
      <p v-if="!executions.length">No matching executions.</p>
      <button v-for="e in executions" :key="e.id" class="list-row" @click="track(e.id)">
        <span>{{ e.customer_id }} · {{ e.state }} · {{ e.current_node_id }}</span
        ><span>{{ e.id.slice(0, 8) }} · {{ e.started_at }}</span>
      </button>
    </section>
    <section v-if="selected" class="panel">
      <h3>Execution inspector</h3>
      <button
        v-if="!readonly && ['RUNNING', 'WAITING'].includes(selected.state)"
        @click="control('cancel')"
      >
        Cancel execution</button
      ><button v-if="admin && selected.state === 'FAILED'" @click="control('retry')">
        Recover failed job
      </button>
      <p>
        {{ selected.id }} · {{ selected.state }} · version {{ version }} ·
        {{ selected.exit_reason || '' }}
      </p>
      <p>
        Entry: {{ selected.started_at }} · Current: {{ selected.current_node_id }} · Correlation /
        trace: {{ selected.id }}
      </p>
      <h4>Transitions &amp; conditions</h4>
      <pre>{{ JSON.stringify(selected.steps, null, 2) }}</pre>
      <h4>Wait deadlines &amp; scheduled jobs</h4>
      <pre>{{ JSON.stringify({ waiters: selected.waiters, jobs: selected.jobs }, null, 2) }}</pre>
      <h4>Messages &amp; decisions</h4>
      <button v-for="d in selected.deliveries" :key="d.id" class="list-row" @click="inspect(d.id)">
        <span>{{ d.node_id }} · {{ d.channel }} · {{ d.status }} · {{ d.attempts }} attempts</span
        ><span>{{ d.decision }}</span>
      </button>
      <details v-if="delivery" open>
        <summary>Delivery attempts and provider result</summary>
        <pre>{{ JSON.stringify(delivery, null, 2) }}</pre>
      </details>
    </section>
  </section>
</template>
<style scoped>
.monitor-canvas {
  height: 560px;
  background: #f4f7f1;
  border: 1px solid #d8e0d2;
  border-radius: 16px;
}
.panel pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 480px;
  overflow: auto;
}
</style>
