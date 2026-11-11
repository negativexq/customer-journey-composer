<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onActivated, onDeactivated, ref, watch } from 'vue';
import { VueFlow, Background, MiniMap, type Node, type Edge } from '@xyflow/vue';
import { api } from '../lib/api';
import WorkflowNode from './WorkflowNode.vue';
import CanvasControls from './CanvasControls.vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import type { Workflow } from '../lib/workflow';
const props = defineProps<{
  journeyId: string;
  readonly?: boolean;
  admin?: boolean;
  initialExecutionId?: string;
  initialDeliveryId?: string;
}>();
const emit = defineEmits<{
  selection: [{ executionID?: string; deliveryID?: string }];
  inspect: [{ kind: 'event' | 'execution' | 'message'; id: string }];
}>();
type Execution = {
  id: string;
  version_id: string;
  entry_event_id: string;
  customer_id: string;
  state: string;
  current_node_id: string;
  started_at: string;
  exit_reason?: string;
  steps: {
    node_id: string;
    state: string;
    output: { port?: string; condition?: boolean; exit_reason?: string; error?: string };
    started_at: string;
    completed_at?: string;
  }[];
  jobs: {
    id: string;
    node_id: string;
    kind: string;
    state: string;
    due_at: string;
    attempts: number;
  }[];
  waiters: { node_id: string; event_name: string; state: string; deadline: string }[];
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
const delivery = ref<{
  id: string;
  provider: string;
  status: string;
  decision: unknown;
  attempt_history: unknown[];
  last_error?: string;
}>();
const stepLabel = (id: string) => workflow.value?.nodes.find((node) => node.id === id)?.label || id;
const time = (at: string) => new Date(at).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC';
function decisionText(output: Execution['steps'][number]['output']) {
  if (output.error) return output.error;
  if (output.condition !== undefined)
    return output.condition
      ? 'Condition matched · Yes branch'
      : 'Condition did not match · No branch';
  if (output.exit_reason) return 'Exit: ' + output.exit_reason.replaceAll('_', ' ');
  return output.port ? 'Continued via ' + output.port : 'Waiting for this step to finish';
}
const stamp = ref('');
let timer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;
let inactive = false;
let refreshQueued = false;
let selectionRequest = 0;
let routeRestore = 0;
let deliveryRequest = 0;
const tracking = ref(false);
const value = (node: string, metric: string) =>
  metrics.value.find((m) => m.node_id === node && m.metric === metric)?.value || 0;
type MonitorNode = Node<InstanceType<typeof WorkflowNode>['$props']['data']>;
const nodes = computed<MonitorNode[]>(
  () =>
    workflow.value?.nodes.map((n, i) => ({
      id: n.id,
      type: 'journey-step',
      position: n.position
        ? { x: n.position.x, y: n.position.y * 1.3 }
        : { x: (i % 3) * 280, y: Math.floor(i / 3) * 220 },
      data: {
        kind: n.type,
        label: n.label,
        config: n.config,
        active:
          selected.value?.current_node_id === n.id &&
          ['RUNNING', 'WAITING'].includes(selected.value.state),
        visited: selected.value?.steps.some((step) => step.node_id === n.id),
        metrics: {
          entered: value(n.id, 'entered'),
          waiting: value(n.id, 'WAITING'),
          completed: value(n.id, 'SUCCEEDED'),
          failed: value(n.id, 'FAILED'),
          skipped: value(n.id, 'SKIPPED'),
          average: value(n.id, 'finished_count')
            ? (value(n.id, 'duration_ms') / value(n.id, 'finished_count') / 1000).toFixed(1) + 's'
            : '—',
        },
      },
    })) || [],
);
const edges = computed<Edge[]>(
  () =>
    workflow.value?.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      sourceHandle: e.port,
      targetHandle: 'in',
      label: { true: 'Yes', false: 'No', matched: 'Matched', timeout: 'Timeout' }[e.port] || '',
      animated: selected.value?.steps?.some(
        (s) => s.node_id === e.source && s.output?.port === e.port,
      ),
      style: {
        stroke: selected.value?.steps?.some(
          (s) => s.node_id === e.source && s.output?.port === e.port,
        )
          ? '#6650e8'
          : '#cbd2df',
      },
    })) || [],
);
async function graph() {
  const requested = version.value;
  const result = await api<{ workflow: Workflow }>(
    `/journeys/${props.journeyId}/versions/${requested}`,
  );
  if (!disposed && requested === version.value) workflow.value = result.workflow;
}
async function refresh() {
  if (disposed || inactive) return;
  if (busy.value) {
    refreshQueued = true;
    return;
  }
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
    const expected = JSON.stringify([customer.value, filter.value, version.value]);
    const [rows, counts] = await Promise.all([
      api<Execution[]>(`/journeys/${props.journeyId}/executions?${params}`),
      api<typeof metrics.value>(`/journeys/${props.journeyId}/node-stats?version=${version.value}`),
    ]);
    if (disposed || inactive) return;
    if (expected !== JSON.stringify([customer.value, filter.value, version.value])) {
      refreshQueued = true;
      return;
    }
    executions.value = rows;
    metrics.value = counts;
    const id = selected.value?.id;
    const request = selectionRequest;
    if (id) {
      const detail = await api<Execution>('/executions/' + id);
      if (!disposed && request === selectionRequest && selected.value?.id === id)
        selected.value = detail;
    }
    stamp.value = new Date().toLocaleTimeString();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
    if (!disposed && !inactive && refreshQueued) {
      refreshQueued = false;
      void refresh();
    } else if (!disposed && !inactive) {
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (poll.value && !document.hidden) void refresh();
        else schedule();
      }, 5000);
    }
  }
}
function schedule() {
  if (disposed || inactive) return;
  clearTimeout(timer);
  timer = setTimeout(() => {
    if (poll.value && !document.hidden) void refresh();
    else schedule();
  }, 5000);
}
async function track(id: string, fromRoute = false) {
  if (!fromRoute) routeRestore++;
  const request = ++selectionRequest;
  deliveryRequest++;
  tracking.value = true;
  selected.value = undefined;
  delivery.value = undefined;
  try {
    const detail = await api<Execution>('/executions/' + id);
    if (disposed || request !== selectionRequest) return;
    selected.value = detail;
    if (!fromRoute) emit('selection', { executionID: id, deliveryID: undefined });
    const v = versions.value.find((v) => v.id === selected.value?.version_id);
    if (v && v.version !== version.value) {
      version.value = v.version;
      await graph();
    }
    await refresh();
  } catch (cause) {
    if (request === selectionRequest) error.value = String(cause);
  } finally {
    if (request === selectionRequest) tracking.value = false;
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
async function inspect(id: string, fromRoute = false) {
  const request = ++deliveryRequest;
  const execution = selected.value?.id;
  try {
    const result = await api<typeof delivery.value>('/deliveries/' + id);
    if (!disposed && request === deliveryRequest && execution === selected.value?.id) {
      delivery.value = result;
      if (!fromRoute) emit('selection', { executionID: execution, deliveryID: id });
    }
  } catch (cause) {
    if (request === deliveryRequest) error.value = String(cause);
  }
}
let activatedBefore = false;
async function restoreSelection() {
  const current = ++routeRestore;
  const id = props.initialExecutionId;
  if (!id) {
    selectionRequest++;
    deliveryRequest++;
    selected.value = undefined;
    delivery.value = undefined;
    return;
  }
  if (selected.value?.id !== id) {
    if (!versions.value.length) await refresh();
    if (current !== routeRestore) return;
    await track(id, true);
  }
  if (current !== routeRestore) return;
  if (props.initialDeliveryId && selected.value && delivery.value?.id !== props.initialDeliveryId)
    await inspect(props.initialDeliveryId, true);
  if (!props.initialDeliveryId) delivery.value = undefined;
}
watch(
  () => [props.initialExecutionId, props.initialDeliveryId],
  () => void restoreSelection(),
);
onMounted(async () => {
  await refresh();
  if (props.initialExecutionId) await restoreSelection();
});
onActivated(() => {
  inactive = false;
  if (activatedBefore) void refresh();
  activatedBefore = true;
});
onDeactivated(() => {
  inactive = true;
  clearTimeout(timer);
});
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
          Refresh every 5s while visible · {{ stamp }}. Purple: visited path; green: current node.
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
        fit-view
        ><Background /><CanvasControls /><MiniMap /><template #node-journey-step="nodeProps"
          ><WorkflowNode v-bind="nodeProps" /></template
      ></VueFlow>
    </div>
    <section class="panel">
      <h3>Executions</h3>
      <p v-if="!executions.length">No matching executions.</p>
      <button v-for="e in executions" :key="e.id" class="list-row" @click="track(e.id)">
        <span>{{ e.customer_id }} · {{ e.state }} · {{ e.current_node_id }}</span
        ><span>{{ e.id.slice(0, 8) }} · {{ e.started_at }}</span>
      </button>
    </section>
    <p v-if="tracking" class="panel" role="status">Loading execution…</p>
    <section v-if="selected" class="panel execution-inspector" aria-label="Execution details">
      <h3>Execution inspector</h3>
      <div class="toolbar">
        <button @click="emit('inspect', { kind: 'execution', id: selected.id })">
          Open activity inspector</button
        ><button
          v-if="selected.entry_event_id"
          @click="emit('inspect', { kind: 'event', id: selected.entry_event_id })"
        >
          View entry event
        </button>
      </div>
      <button
        v-if="!readonly && ['RUNNING', 'WAITING'].includes(selected.state)"
        @click="control('cancel')"
      >
        Cancel execution</button
      ><button v-if="admin && selected.state === 'FAILED'" @click="control('retry')">
        Recover failed job
      </button>
      <div class="execution-overview">
        <StatusBadge :status="selected.state" /><span>Version {{ version }}</span
        ><span>{{ selected.customer_id }}</span
        ><span v-if="selected.exit_reason"
          >Exit: {{ selected.exit_reason.replaceAll('_', ' ') }}</span
        >
      </div>
      <dl class="execution-facts">
        <div>
          <dt>Entered</dt>
          <dd>{{ time(selected.started_at) }}</dd>
        </div>
        <div>
          <dt>Current step</dt>
          <dd>{{ stepLabel(selected.current_node_id) }}</dd>
        </div>
        <div>
          <dt>Execution / correlation ID</dt>
          <dd>{{ selected.id }}</dd>
        </div>
      </dl>
      <h4>Transitions &amp; conditions</h4>
      <div class="execution-timeline">
        <div
          v-for="step in selected.steps"
          :key="step.node_id"
          class="timeline-step"
          :class="step.state.toLowerCase()"
        >
          <span class="timeline-node"
            ><AppIcon
              :name="
                step.state === 'WAITING' ? 'clock' : step.state === 'FAILED' ? 'close' : 'check'
              "
              :size="15"
          /></span>
          <div class="timeline-step-content">
            <div>
              <strong>{{ stepLabel(step.node_id) }}</strong
              ><StatusBadge :status="step.state" />
            </div>
            <p>{{ decisionText(step.output) }}</p>
            <time
              >{{ time(step.started_at)
              }}<span v-if="step.completed_at">
                · finished {{ time(step.completed_at) }}</span
              ></time
            >
          </div>
        </div>
      </div>
      <h4>Wait deadlines &amp; scheduled jobs</h4>
      <p
        v-if="
          !selected.waiters.some((w) => w.state === 'WAITING') &&
          !selected.jobs.some((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))
        "
        class="muted"
      >
        No pending waits or scheduled work.
      </p>
      <div
        v-for="waiter in selected.waiters.filter((w) => w.state === 'WAITING')"
        :key="waiter.node_id"
        class="execution-wait"
      >
        <AppIcon name="events" :size="17" />
        <div>
          <strong>Waiting for {{ waiter.event_name }}</strong>
          <p>{{ stepLabel(waiter.node_id) }} · until {{ time(waiter.deadline) }}</p>
        </div>
      </div>
      <div
        v-for="job in selected.jobs.filter((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))"
        :key="job.id"
        class="execution-wait"
      >
        <AppIcon name="clock" :size="17" />
        <div>
          <strong>{{ stepLabel(job.node_id) }}</strong>
          <p>{{ job.kind }} · due {{ time(job.due_at) }} · {{ job.attempts }} attempts</p>
        </div>
        <StatusBadge :status="job.state" />
      </div>
      <h4>Messages &amp; decisions</h4>
      <p v-if="!selected.deliveries.length" class="muted">
        No logical messages created for this execution.
      </p>
      <button v-for="d in selected.deliveries" :key="d.id" class="list-row" @click="inspect(d.id)">
        <span
          >{{ stepLabel(d.node_id) }} · {{ d.channel.toUpperCase() }} · {{ d.status }} ·
          {{ d.attempts }} attempts</span
        ><AppIcon name="arrow" :size="15" />
      </button>
      <section v-if="delivery" class="delivery-inspector">
        <div class="surface-heading">
          <h4>Delivery decision &amp; attempts</h4>
          <button @click="inspect(delivery.id)">Refresh delivery</button>
        </div>
        <DecisionSummary :decision="delivery.decision" /><AttemptHistory
          :attempts="delivery.attempt_history"
          :provider="delivery.provider"
          :response="delivery.last_error"
        />
      </section>
      <details class="execution-raw">
        <summary>Advanced execution payload</summary>
        <pre>{{ JSON.stringify(selected, null, 2) }}</pre>
      </details>
    </section>
  </section>
</template>
<style scoped>
.execution-inspector h4 {
  font-size: 13px;
  margin: 22px 0 16px;
}
.execution-overview {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  margin: 18px 0;
  font-size: 13px;
  color: var(--muted);
}
.execution-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  padding: 16px;
  background: #fafbfe;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.execution-facts dt {
  color: var(--muted);
  font-size: 12px;
}
.execution-facts dd {
  margin: 6px 0 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.execution-wait {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-bottom: 10px;
  font-size: 13px;
}
.execution-wait p {
  font-size: 12px;
  color: var(--muted);
  margin: 6px 0 0;
}
.execution-raw {
  margin-top: 24px;
  font-size: 12px;
  color: var(--muted);
}
.execution-raw summary {
  cursor: pointer;
}
.delivery-inspector {
  margin-top: 16px;
}
.delivery-inspector .surface-heading {
  padding: 0;
}
@media (max-width: 700px) {
  .execution-facts {
    grid-template-columns: minmax(0, 1fr);
  }
}

.monitor-canvas {
  height: 560px;
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 10px;
}
.panel pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 480px;
  overflow: auto;
}
</style>
