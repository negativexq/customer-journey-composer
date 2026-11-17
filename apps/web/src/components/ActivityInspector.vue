<script setup lang="ts">
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { errorMessage, formatDateTime } from '../lib/format';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { api } from '../lib/api';
import type { InspectionKind } from '../lib/navigation';
import DetailDrawer from './ui/DetailDrawer.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import type { DeliveryEngagement } from '../lib/engagement';
import JourneyRoute from './JourneyRoute.vue';
import { journeyPath, type ExecutionDetail } from '../lib/customer-journey';
import { loadDefinition, type Definition } from '../lib/journey-definition';
import { stepSummary } from '../lib/workflow-presentation';
import AppIcon from './ui/AppIcon.vue';
import { channelIcons, channelNames, type Channel } from '../lib/workflow';
type ExecutionLink = { id: string; journey_id: string; state: string; started_at: string };
type Detail = {
  engagement?: DeliveryEngagement[];
  id?: string;
  event_id?: string;
  event_name?: string;
  customer_id?: string;
  journey_id?: string;
  version_id?: string;
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
  deliveries?: {
    id: string;
    node_id: string;
    channel: string;
    status: string;
    accepted_at?: string | null;
    decision?: { reason?: string } | null;
  }[];
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
// What an execution needs to read as a route: the published definition and the journey's name.
const definition = ref<Definition>();
const journeyName = ref('');
// A message row reads as the step that sent it, not its internal ID.
function stepLabel(nodeID: string) {
  return definition.value?.workflow.nodes.find((n) => n.id === nodeID)?.label || nodeID;
}
const blockReasons: Record<string, string> = {
  consent_denied: 'no channel permission',
  consent_revoked: 'permission withdrawn',
  frequency_cap: 'sending limit reached',
  execution_terminated: 'journey ended first',
  missing_personalization: 'personalization missing',
  invalid_recipient: 'no valid recipient',
  recipient_changed: 'recipient changed',
};
function messageNote(message: NonNullable<Detail['deliveries']>[number]) {
  const channel = channelNames[message.channel as Channel] || message.channel;
  const reason = message.decision?.reason;
  if (message.status === 'BLOCKED' && reason)
    return `${channel} · ${blockReasons[reason] || reason.replaceAll('_', ' ')}`;
  if (message.accepted_at) return `${channel} · ${formatDateTime(message.accepted_at)}`;
  return channel;
}
const path = computed(() =>
  props.kind === 'execution' && detail.value?.steps
    ? journeyPath(
        definition.value?.workflow || { schema_version: 1, nodes: [], edges: [] },
        detail.value as unknown as ExecutionDetail,
        stepSummary,
      )
    : undefined,
);
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
// Opening a linked record from inside the drawer leaves a trail, so the person can step
// back to where they came from. Each step is also a browser history entry, so the back
// button here and the browser's back button take the same step.
type Stop = { kind: InspectionKind; id: string; title: string };
const trail = ref<Stop[]>([]);
let forward = false;
function go(next: { kind: InspectionKind; id: string }) {
  if (props.kind && props.recordId)
    trail.value.push({ kind: props.kind, id: props.recordId, title: title.value });
  forward = true;
  emit('inspect', next);
}
function back() {
  if (trail.value.length) window.history.back();
}
watch(
  () => [props.kind, props.recordId] as const,
  ([kind, id]) => {
    const last = trail.value.at(-1);
    if (forward) forward = false;
    else if (last && last.kind === kind && last.id === id) trail.value.pop();
    else trail.value = [];
  },
);
const previous = computed(() => trail.value.at(-1));
const time = (value?: string) =>
  value ? new Date(value).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC' : 'Not recorded';
async function load() {
  controller?.abort();
  controller = new AbortController();
  const current = ++sequence;
  detail.value = undefined;
  definition.value = undefined;
  journeyName.value = '';
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
    if (props.kind === 'execution' && result.journey_id) {
      // The route still reads without these, with step ids instead of names.
      const journeyID = result.journey_id;
      void api<{ name: string }>('/journeys/' + journeyID)
        .then((journey) => {
          if (current === sequence) journeyName.value = journey.name;
        })
        .catch(() => undefined);
      if (result.version_id)
        void loadDefinition(journeyID, result.version_id)
          .then((loaded) => {
            if (current === sequence) definition.value = loaded;
          })
          .catch(() => undefined);
    }
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
    if (current === sequence && !controller.signal.aborted) error.value = errorMessage(cause);
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
    <template v-if="previous" #back
      ><button type="button" class="activity-back" @click="back">
        <AppIcon name="back" :size="14" />{{ previous.title }}
      </button></template
    >
    <SkeletonBlock v-if="busy" label="Loading activity…" variant="text" :rows="6" />
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
            journeyName ||
            'Journey execution'
          }}
        </h3>
        <StatusBadge :status="detail.status || detail.state || 'UNKNOWN'" />
      </div>
      <p v-if="path" class="activity-headline">
        {{ path.headline }}<template v-if="path.duration"> · took {{ path.duration }}</template>
      </p>
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
        <div v-if="definition">
          <dt>Version</dt>
          <dd>Version {{ definition.version }}</dd>
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
          @click="go({ kind: 'event', id: detail.entry_event_id })"
        >
          View entry event</button
        ><button
          v-if="detail.execution_id"
          @click="go({ kind: 'execution', id: detail.execution_id })"
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
          @click="go({ kind: 'execution', id: execution.id })"
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
        ><JourneyRoute
          v-if="path"
          class="activity-route"
          :path="path"
          @message="go({ kind: 'message', id: $event })"
        />
        <h3>Messages</h3>
        <p v-if="!detail.deliveries?.length" class="muted">No messages created.</p>
        <div v-else class="activity-messages">
          <button
            v-for="message in detail.deliveries"
            :key="message.id"
            class="activity-message"
            @click="go({ kind: 'message', id: message.id })"
          >
            <span class="channel-icon" :class="message.channel" aria-hidden="true"
              ><AppIcon :name="channelIcons[message.channel as Channel] || 'message'" :size="15"
            /></span>
            <span class="activity-message-copy"
              ><strong>{{ stepLabel(message.node_id) }}</strong
              ><small>{{ messageNote(message) }}</small></span
            >
            <StatusBadge :status="message.status" />
            <AppIcon name="arrow" :size="14" class="activity-message-go" />
          </button>
        </div>
      </template>
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
          :engagement="detail.engagement"
      /></template>
      <details class="activity-advanced">
        <summary>Advanced payload</summary>
        <pre>{{ JSON.stringify(detail, null, 2) }}</pre>
      </details>
    </template>
  </DetailDrawer>
</template>
<style scoped>
.activity-back {
  justify-self: start;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 0 8px 0 4px;
  margin-left: -4px;
  border: 0;
  border-radius: 7px;
  background: none;
  color: var(--accent-text);
  font-size: 12px;
  font-weight: 600;
}
.activity-back:hover {
  background: var(--accent-soft);
}
.activity-messages {
  display: grid;
  gap: 8px;
}
.activity-message {
  display: grid;
  grid-template-columns: 31px minmax(0, 1fr) auto 14px;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 56px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  text-align: left;
}
.activity-message:hover {
  border-color: var(--violet-87);
  background: var(--gray-99);
}
.activity-message:hover .activity-message-go {
  color: var(--accent-text);
  transform: translateX(2px);
}
.activity-message-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.activity-message-copy strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 600;
}
.activity-message-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: var(--muted);
}
.activity-message-go {
  color: var(--muted);
  transition:
    transform 140ms ease-out,
    color 140ms ease-out;
}
@media (prefers-reduced-motion: reduce) {
  .activity-message-go {
    transition: none;
  }
}
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
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 20px;
  margin: 20px 0;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--page);
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
.activity-headline {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--muted);
}
.activity-route {
  margin-top: 24px;
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
  background: var(--page);
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
