<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{
  journeyId?: string;
  beforeRun?: () => Promise<void>;
  readonly?: boolean;
  nodeId?: string;
}>();
type Trace = { at: string; kind: string; node_id?: string; message: string };
type State = {
  now: string;
  status: string;
  result: string;
  paused: boolean;
  finalized: boolean;
  current_node_id?: string;
  exit_reason?: string;
  trace: Trace[];
  steps: {
    node_id: string;
    label: string;
    state: string;
    started_at: string;
    deadline?: string;
    output: unknown;
  }[];
  messages: {
    id: string;
    channel: string;
    status: string;
    scheduled_at: string;
    decision: unknown;
  }[];
  assertions: { description: string; pass: boolean; actual: unknown }[];
};
type Run = { id: string; revision: number; workflow_hash: string; state: State };
const journeys = ref<Journey[]>([]);
const journey = ref(props.journeyId || '');
const version = ref(0);
const name = ref('Customer journey scenario');
const start = ref(new Date().toISOString().slice(0, 19));
const mode = ref('full');
const node = ref(props.nodeId || '');
const customerID = ref('');
const profile = ref('Custom Test Customer');
const customer = ref(
  JSON.stringify(
    {
      email: 'sandbox@example.invalid',
      phone: '+905551234567',
      locale: 'en',
      timezone: 'Europe/Istanbul',
      attributes: { first_name: 'Ada', purchased: false, total_orders: 0, cart_amount: 2450 },
      consents: { email: true, sms: true },
    },
    null,
    2,
  ),
);
const entry = ref(
  '{"event_id":"sandbox_entry","event_name":"cart_updated","properties":{"amount":2450,"currency":"TRY"}}',
);
const events = ref('[]');
const changes = ref('[]');
const assertions = ref(
  '[{"type":"state","expected":"COMPLETED"},{"type":"no_duplicate_delivery","expected":true}]',
);
const faults = ref('[]');
const faultChannel = ref('email');
const prior = ref(0);
const scenarios = ref<{ id: string; name: string; journey_id: string }[]>([]);
const savedScenario = ref('');
const currentDraft = ref(false);
const runs = ref<{ id: string; result: string; execution_state: string; created_at: string }[]>([]);
const run = ref<Run>();
const path = ref<State>();
const error = ref('');
const notice = ref('');
const busy = ref(false);
const state = computed(() => path.value || run.value?.state);
function preset() {
  const data = JSON.parse(customer.value);
  data.attributes.total_orders = profile.value === 'Returning Customer' ? 3 : 0;
  data.attributes.inactive = profile.value === 'Inactive Customer';
  if (profile.value === 'New Customer') data.attributes.first_name = 'New customer';
  customer.value = JSON.stringify(data, null, 2);
}
function input() {
  return {
    journey_id: journey.value || undefined,
    version: version.value,
    scenario_id: savedScenario.value || undefined,
    use_current_draft: currentDraft.value,
    ...(savedScenario.value
      ? {}
      : {
          scenario: {
            name: name.value,
            mode: mode.value,
            node_id: node.value || undefined,
            start_time: new Date(start.value + 'Z').toISOString(),
            customer_id: customerID.value || undefined,
            customer: JSON.parse(customer.value),
            entry_event: JSON.parse(entry.value),
            events: JSON.parse(events.value),
            attribute_changes: JSON.parse(changes.value),
            assertions: JSON.parse(assertions.value),
            provider_faults: JSON.parse(faults.value),
            fault_channel: faultChannel.value,
            prior_messages: prior.value,
          },
        }),
  };
}
async function refresh() {
  [scenarios.value, runs.value] = await Promise.all([
    api<typeof scenarios.value>('/test-scenarios'),
    api<typeof runs.value>('/test-runs' + (journey.value ? '?journey_id=' + journey.value : '')),
  ]);
}
async function perform(action: () => Promise<void>) {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await action();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
async function prepare() {
  if (props.beforeRun) await props.beforeRun();
}
async function create() {
  await perform(async () => {
    await prepare();
    run.value = await api<Run>('/test-runs', 'POST', input());
    path.value = undefined;
    await refresh();
  });
}
async function saveScenario() {
  await perform(async () => {
    await prepare();
    await api('/test-scenarios', 'POST', input());
    notice.value = 'Scenario saved with workflow and customer snapshots';
    await refresh();
  });
}
async function preview() {
  await perform(async () => {
    await prepare();
    path.value = await api<State>('/preview/journey', 'POST', input());
  });
}
async function advance(operation: string, seconds = 0) {
  await perform(async () => {
    if (!run.value) return;
    path.value = undefined;
    run.value = await api<Run>(`/test-runs/${run.value.id}/advance`, 'POST', {
      operation,
      seconds,
      revision: run.value.revision,
    });
    await refresh();
  });
}
async function replay() {
  await perform(async () => {
    if (!run.value) return;
    run.value = await api<Run>(`/test-runs/${run.value.id}/replay`, 'POST');
    path.value = undefined;
    await refresh();
  });
}
async function open(id: string) {
  await perform(async () => {
    run.value = await api<Run>('/test-runs/' + id);
    path.value = undefined;
  });
}
onMounted(
  () =>
    void perform(async () => {
      if (!props.journeyId) journeys.value = await api<Journey[]>('/journeys?limit=200');
      await refresh();
    }),
);
</script>
<template>
  <section class="panel test-lab">
    <h2>Test Lab</h2>
    <p>
      Isolated simulation · virtual clock · no provider calls. Times below are UTC. Profiles are
      copied; live customers are never edited.
    </p>
    <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    <p role="status">{{ notice }}</p>
    <div class="editor-form">
      <label v-if="!journeyId"
        >Journey<select v-model="journey" @change="refresh">
          <option value="">Select journey</option>
          <option v-for="j in journeys" :key="j.id" :value="j.id">{{ j.name }}</option>
        </select></label
      >
      <label
        >Saved scenario<select v-model="savedScenario">
          <option value="">New scenario</option>
          <option
            v-for="s in scenarios.filter((s) => !journey || s.journey_id === journey)"
            :key="s.id"
            :value="s.id"
          >
            {{ s.name }}
          </option>
        </select></label
      >
      <label
        >Published version (0 = draft)<input v-model.number="version" type="number" min="0"
      /></label>
      <label v-if="savedScenario"
        ><input v-model="currentDraft" type="checkbox" /> Regression: use current draft with saved
        inputs</label
      >
    </div>
    <details :open="!savedScenario">
      <summary>Scenario inputs</summary>
      <div class="editor-form">
        <label>Scenario name<input v-model="name" maxlength="160" /></label
        ><label>Start time (UTC)<input v-model="start" type="datetime-local" step="1" /></label
        ><label
          >Test mode<select v-model="mode">
            <option value="full">Full journey / event sequence</option>
            <option value="single_node">Single node</option>
          </select></label
        ><label v-if="mode === 'single_node'">Node ID<input v-model="node" /></label>
        <label
          >Preview as customer<select v-model="profile" @change="preset">
            <option>Custom Test Customer</option>
            <option>Existing Customer</option>
            <option>New Customer</option>
            <option>Returning Customer</option>
            <option>Inactive Customer</option>
          </select></label
        ><label v-if="profile === 'Existing Customer'"
          >Source customer ID<input v-model="customerID" /></label
        ><label
          >Prior sandbox messages<input
            v-model.number="prior"
            type="number"
            min="0"
            max="1000" /></label
        ><label
          >Fault channel<select v-model="faultChannel">
            <option>email</option>
            <option>sms</option>
          </select></label
        ><label
          >Provider faults<select v-model="faults">
            <option value="[]">Success</option>
            <option value="[429,500,200]">429 → 500 → success</option>
            <option value="[408]">Ambiguous timeout</option>
            <option value="[400]">Permanent failure</option>
          </select></label
        >
      </div>
      <div class="test-input-grid">
        <label
          >Customer overrides (JSON)<textarea
            v-model="customer"
            rows="12"
            spellcheck="false"
          /></label
        ><label>Entry event (JSON)<textarea v-model="entry" rows="6" spellcheck="false" /></label
        ><label
          >Event sequence (JSON; after_seconds, event)<textarea
            v-model="events"
            rows="6"
            spellcheck="false"
          /></label
        ><label
          >Attribute changes (JSON; after_seconds, attributes)<textarea
            v-model="changes"
            rows="6"
            spellcheck="false"
          /></label
        ><label
          >Assertions (JSON)<textarea v-model="assertions" rows="6" spellcheck="false" />
        </label>
      </div>
    </details>
    <div class="detail-tabs">
      <button :disabled="busy || readonly || (!journey && !savedScenario)" @click="create">
        Create test run</button
      ><button :disabled="busy || readonly" @click="saveScenario">Save scenario</button
      ><button :disabled="busy" @click="preview">Preview journey path</button>
    </div>
    <div v-if="run" class="panel">
      <p>Run {{ run.id }} · snapshot {{ run.workflow_hash.slice(0, 12) }}</p>
      <div class="detail-tabs">
        <button
          v-for="[label, op, seconds] in [
            ['Run', 'run', 0],
            ['Step', 'step', 0],
            ['Advance 1h', 'advance', 3600],
            ['Advance 1d', 'advance', 86400],
            ['Run until next event', 'until_next', 0],
            ['Run to completion', 'complete', 0],
            ['Pause', 'pause', 0],
            ['Resume', 'resume', 0],
          ] as const"
          :key="label"
          :disabled="busy || readonly"
          @click="advance(op, seconds)"
        >
          {{ label }}</button
        ><button :disabled="busy || readonly" @click="replay">Reset / Replay</button>
      </div>
    </div>
    <section v-if="state" aria-live="polite">
      <h3>
        {{ path ? 'Journey path preview' : 'Run result' }}:
        <span data-testid="test-result">{{ state.result }}</span>
      </h3>
      <p>
        {{ state.status }} {{ state.paused ? '· paused' : '' }} · {{ state.now }} · Node
        {{ state.current_node_id || '—' }} · {{ state.exit_reason || '' }}
      </p>
      <p v-for="(a, i) in state.assertions" :key="i">
        {{ a.pass ? '✓' : '✗' }} {{ a.description }} · actual {{ a.actual }}
      </p>
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Node</th>
              <th>State</th>
              <th>Started (UTC)</th>
              <th>Deadline</th>
              <th>Decision</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(s, i) in state.steps" :key="i">
              <td>{{ s.label }} ({{ s.node_id }})</td>
              <td>{{ s.state }}</td>
              <td>{{ s.started_at }}</td>
              <td>{{ s.deadline }}</td>
              <td>
                <details>
                  <summary>Inspect</summary>
                  <pre>{{ JSON.stringify(s.output, null, 2) }}</pre>
                </details>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <h3>Sandbox message decisions</h3>
      <p v-if="!state.messages.length">No messages produced.</p>
      <p v-for="m in state.messages" :key="m.id">
        {{ m.scheduled_at }} · {{ m.channel }} · {{ m.status }} · {{ m.id }} · {{ m.decision }}
      </p>
      <details>
        <summary>Execution trace ({{ state.trace.length }})</summary>
        <p v-for="(t, i) in state.trace" :key="i">
          {{ t.at }} · {{ t.kind }} · {{ t.node_id }} · {{ t.message }}
        </p>
      </details>
    </section>
    <h3>Recent runs</h3>
    <p v-if="!runs.length">No saved runs.</p>
    <button v-for="r in runs" :key="r.id" class="list-row" @click="open(r.id)">
      <span>{{ r.id.slice(0, 8) }} · {{ r.execution_state }} · {{ r.result }}</span
      ><time>{{ new Date(r.created_at).toLocaleString() }}</time>
    </button>
  </section>
</template>
<style scoped>
.test-input-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr));
  gap: 16px;
}
.test-lab textarea {
  font-family: monospace;
}
.test-lab pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.test-lab .detail-tabs {
  flex-wrap: wrap;
}
.test-lab table {
  width: 100%;
  text-align: left;
}
.table-scroll {
  overflow: auto;
}
</style>
