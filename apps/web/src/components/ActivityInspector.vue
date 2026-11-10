<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { api } from '../lib/api';
import type { InspectionKind } from '../lib/navigation';
import DetailDrawer from './ui/DetailDrawer.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
type ExecutionLink = { id: string; journey_id: string; state: string; started_at: string };
type Detail = {
  id?: string;
  event_id?: string;
  event_name?: string;
  customer_id?: string;
  journey_id?: string;
  execution_id?: string;
  entry_event_id?: string;
  state?: string;
  status?: string;
  mode?: string;
  channel?: string;
  node_id?: string;
  started_at?: string;
  created_at?: string;
  occurred_at?: string;
  exit_reason?: string;
  properties?: unknown;
  message?: { subject?: string; text?: string; html?: string; to?: string };
  decision?: unknown;
  attempt_history?: unknown[];
  provider?: string;
  last_error?: string;
  steps?: {
    node_id: string;
    state: string;
    started_at: string;
    output?: { condition?: boolean; port?: string; exit_reason?: string; error?: string };
  }[];
  deliveries?: { id: string; node_id: string; channel: string; status: string }[];
};
const props = defineProps<{ kind?: InspectionKind; recordId?: string }>();
const emit = defineEmits<{
  close: [];
  inspect: [{ kind: InspectionKind; id: string }];
  customer: [string];
  monitor: [{ journeyID: string; executionID: string }];
}>();
const detail = ref<Detail>();
const executions = ref<ExecutionLink[]>([]);
const error = ref('');
const busy = ref(false);
let sequence = 0;
let controller: AbortController | undefined;
const title = computed(
  () =>
    ({ event: 'Event details', execution: 'Execution details', message: 'Message details' })[
      props.kind || 'event'
    ],
);
const time = (value?: string) =>
  value ? new Date(value).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC' : 'Not recorded';
async function load() {
  controller?.abort();
  controller = new AbortController();
  const current = ++sequence;
  detail.value = undefined;
  executions.value = [];
  error.value = '';
  if (!props.kind || !props.recordId) {
    busy.value = false;
    return;
  }
  busy.value = true;
  const endpoint = { event: 'events', execution: 'executions', message: 'deliveries' }[props.kind];
  try {
    const result = await api<Detail>(
      `/${endpoint}/${encodeURIComponent(props.recordId)}`,
      'GET',
      undefined,
      {},
      30000,
      controller.signal,
    );
    if (current !== sequence) return;
    detail.value = result;
    if (props.kind === 'event') {
      const rows = await api<ExecutionLink[]>(
        `/events/${encodeURIComponent(props.recordId)}/executions?limit=25`,
        'GET',
        undefined,
        {},
        30000,
        controller.signal,
      );
      if (current === sequence) executions.value = rows;
    }
  } catch (cause) {
    if (current === sequence && !controller.signal.aborted) error.value = String(cause);
  } finally {
    if (current === sequence) busy.value = false;
  }
}
watch(() => [props.kind, props.recordId], load, { immediate: true });
onBeforeUnmount(() => {
  sequence++;
  controller?.abort();
});
</script>
<template>
  <DetailDrawer :model-value="!!kind && !!recordId" :title="title" @close="emit('close')">
    <p v-if="busy" role="status" aria-busy="true">Loading activity…</p>
    <div v-if="error" class="inline-alert error-alert" role="alert">
      <p>{{ error }}</p>
      <button @click="load">Retry activity</button>
    </div>
    <template v-if="detail">
      <div class="activity-summary">
        <h3>
          {{
            detail.event_name ||
            detail.message?.subject ||
            detail.channel?.toUpperCase() ||
            'Journey execution'
          }}
        </h3>
        <StatusBadge :status="detail.status || detail.state || 'UNKNOWN'" />
      </div>
      <dl class="activity-facts">
        <div>
          <dt>Record</dt>
          <dd>{{ detail.event_id || detail.id }}</dd>
        </div>
        <div v-if="detail.customer_id">
          <dt>Customer</dt>
          <dd>
            <button class="text-button" @click="emit('customer', detail.customer_id)">
              {{ detail.customer_id }}
            </button>
          </dd>
        </div>
        <div>
          <dt>Time</dt>
          <dd>{{ time(detail.occurred_at || detail.started_at || detail.created_at) }}</dd>
        </div>
        <div v-if="detail.mode">
          <dt>Mode</dt>
          <dd>{{ detail.mode }}</dd>
        </div>
        <div v-if="detail.exit_reason">
          <dt>Exit reason</dt>
          <dd>{{ detail.exit_reason.replaceAll('_', ' ') }}</dd>
        </div>
      </dl>
      <div class="toolbar">
        <button
          v-if="detail.entry_event_id"
          @click="emit('inspect', { kind: 'event', id: detail.entry_event_id })"
        >
          View entry event</button
        ><button
          v-if="detail.execution_id"
          @click="emit('inspect', { kind: 'execution', id: detail.execution_id })"
        >
          View execution</button
        ><button
          v-if="kind === 'execution' && detail.journey_id && detail.id"
          @click="emit('monitor', { journeyID: detail.journey_id, executionID: detail.id })"
        >
          Open in Live Monitor
        </button>
      </div>
      <template v-if="kind === 'event'"
        ><h3>Executions started by this event</h3>
        <p v-if="!busy && !executions.length" class="muted">
          No executions started by this event. Events can also resolve waits or exit existing
          executions.
        </p>
        <button
          v-for="execution in executions"
          :key="execution.id"
          class="list-row"
          @click="emit('inspect', { kind: 'execution', id: execution.id })"
        >
          <span>{{ execution.id.slice(0, 8) }} · {{ time(execution.started_at) }}</span
          ><StatusBadge :status="execution.state" /></button
        ><small v-if="executions.length"
          >Showing the latest {{ executions.length }} · up to 25.</small
        >
        <h3>Event properties</h3>
        <pre>{{ JSON.stringify(detail.properties, null, 2) }}</pre>
      </template>
      <template v-if="kind === 'execution'"
        ><h3>Step history</h3>
        <ol class="activity-timeline">
          <li v-for="step in detail.steps" :key="step.node_id">
            <div>
              <strong>{{ step.node_id }}</strong
              ><StatusBadge :status="step.state" />
            </div>
            <p v-if="step.output?.condition !== undefined">
              Condition {{ step.output.condition ? 'matched · Yes' : 'did not match · No' }}
            </p>
            <p v-else-if="step.output?.error">{{ step.output.error }}</p>
            <p v-else-if="step.output?.port">Continued via {{ step.output.port }}</p>
            <time>{{ time(step.started_at) }}</time>
          </li>
        </ol>
        <h3>Messages</h3>
        <p v-if="!detail.deliveries?.length" class="muted">No logical messages created.</p>
        <button
          v-for="message in detail.deliveries"
          :key="message.id"
          class="list-row"
          @click="emit('inspect', { kind: 'message', id: message.id })"
        >
          <span>{{ message.node_id }} · {{ message.channel.toUpperCase() }}</span
          ><StatusBadge :status="message.status" /></button
      ></template>
      <template v-if="kind === 'message'"
        ><h3>Rendered message</h3>
        <p v-if="detail.message?.to">To: {{ detail.message.to }}</p>
        <pre>{{
          detail.message?.text ||
          'No text fallback recorded. Inspect the advanced payload for the HTML source.'
        }}</pre>
        <DecisionSummary :decision="detail.decision" />
        <h3>Provider attempts</h3>
        <AttemptHistory
          :attempts="detail.attempt_history || []"
          :provider="detail.provider"
          :response="detail.last_error"
      /></template>
      <details class="activity-advanced">
        <summary>Advanced payload</summary>
        <pre>{{ JSON.stringify(detail, null, 2) }}</pre>
      </details>
    </template>
  </DetailDrawer>
</template>
<style scoped>
.activity-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
h3 {
  font-size: 14px;
  margin: 24px 0 14px;
}
.activity-summary h3 {
  font-size: 18px;
  margin: 0;
}
.activity-facts {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 14px;
  margin: 24px 0;
  font-size: 13px;
}
.activity-facts dt {
  color: var(--muted);
  font-size: 12px;
  margin-bottom: 6px;
}
.activity-facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.activity-timeline {
  list-style: none;
  padding: 0;
  display: grid;
  gap: 12px;
}
.activity-timeline li {
  border-left: 2px solid var(--border);
  padding: 4px 0 8px 16px;
  font-size: 13px;
}
.activity-timeline li > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
time,
small {
  font-size: 12px;
  color: var(--muted);
}
pre {
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  background: #f8fafc;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  max-height: 440px;
  overflow: auto;
}
.activity-advanced {
  margin-top: 24px;
  font-size: 13px;
}
summary {
  cursor: pointer;
}
</style>
