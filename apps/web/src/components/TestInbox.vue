<script setup lang="ts">
import { runTitle, type TestRunSummary } from '../lib/test-runs';
import EmptyState from './ui/EmptyState.vue';
import { errorMessage, formatDateTime } from '../lib/format';
import { computed, onMounted, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import { api, type Journey } from '../lib/api';
type Row = {
  id: string;
  test_run_id?: string;
  delivery_id?: string;
  journey_id?: string;
  journey_name?: string;
  customer_id?: string;
  node_label: string;
  source_node?: string;
  channel: string;
  mode: string;
  status: string;
  simulated_at: string;
  created_at: string;
  version?: number;
};
type Detail = Row & {
  message: {
    subject?: string;
    preheader?: string;
    html?: string;
    text?: string;
    to?: string;
    personalization_values?: Record<string, string>;
    template_id?: string;
    encoding?: string;
    segments?: number;
  };
  decision: unknown;
  attempt_history: unknown[];
  provider_response?: string;
  provider_status?: string;
  provider?: string;
  execution_trace: unknown[];
  trace_id: string;
  workflow_hash?: string;
};
const props = defineProps<{
  readonly?: boolean;
  embedded?: boolean;
  testRunId?: string;
  runRevision?: number;
}>();
const rows = ref<Row[]>([]);
const selected = ref<Detail>();
const previous = ref<unknown>();
const runs = ref<TestRunSummary[]>([]);
const journeys = ref<Journey[]>([]);
const run = ref(props.testRunId || '');
let detailSequence = 0;
let loadSequence = 0;
const customer = ref('');
const journey = ref('');
const channel = ref('');
const status = ref('');
const error = ref('');
const notice = ref('');
const busy = ref(false);
const tab = ref('Preview');
const device = ref('desktop');
const customers = computed(() => [
  ...new Set(rows.value.map((r) => r.customer_id).filter((c): c is string => !!c)),
]);
const escape = (s: string) =>
  s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const document = computed(
  () =>
    `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none';style-src 'unsafe-inline';img-src data:;form-action 'none';base-uri 'none'"><meta name="viewport" content="width=device-width"><style>body{font:16px/1.7 Arial;padding:24px}img{max-width:100%}</style></head><body>${selected.value?.message.html || '<pre>' + escape(selected.value?.message.text || '') + '</pre>'}</body></html>`,
);
async function perform(action: () => Promise<void>) {
  busy.value = true;
  error.value = '';
  try {
    await action();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
async function load(more = false) {
  const request = ++loadSequence;
  await perform(async () => {
    const params = new URLSearchParams();
    if (run.value) params.set('test_run_id', run.value);
    if (customer.value) params.set('customer_id', customer.value);
    if (journey.value) params.set('journey_id', journey.value);
    if (channel.value) params.set('channel', channel.value);
    if (status.value) params.set('status', status.value);
    if (more && rows.value.length) {
      const last = rows.value.at(-1)!;
      params.set('before', last.created_at);
      params.set('before_id', last.id);
    }
    const result = await api<Row[]>('/test-inbox?' + params);
    if (request !== loadSequence) return;
    rows.value = more ? [...rows.value, ...result] : result;
    if (props.embedded) {
      const row = rows.value.find((row) => row.id === selected.value?.id) || rows.value[0];
      if (row) await open(row);
      else selected.value = undefined;
    }
  });
}
async function open(row: Row) {
  const request = ++detailSequence;
  await perform(async () => {
    const detail = await api<Detail>('/test-messages/' + row.id);
    if (request !== detailSequence) return;
    selected.value = detail;
    previous.value = undefined;
    tab.value = 'Preview';
  });
}
async function replay() {
  await perform(async () => {
    if (!selected.value?.test_run_id) return;
    const next = await api<{ id: string; revision: number }>(
      '/test-runs/' + selected.value.test_run_id + '/replay',
      'POST',
    );
    await api('/test-runs/' + next.id + '/advance', 'POST', {
      operation: 'complete',
      revision: next.revision,
    });
    run.value = next.id;
    notice.value = 'Replayed original snapshot; new messages are shown below.';
  });
  await load();
}
async function compare() {
  await perform(async () => {
    if (!selected.value) return;
    previous.value = await api('/test-messages/' + selected.value.id + '/previous');
    tab.value = 'Compare';
  });
}
async function copy() {
  await perform(async () => {
    await navigator.clipboard.writeText(
      selected.value?.message.html || selected.value?.message.text || '',
    );
    notice.value = 'Rendered content copied';
  });
}
async function filterCustomer(value: string) {
  customer.value = value;
  await load();
}
watch(
  () => [props.testRunId, props.runRevision] as const,
  ([id]) => {
    if (!props.embedded || !id) return;
    if (run.value !== id) {
      detailSequence++;
      selected.value = undefined;
      rows.value = [];
    }
    run.value = id;
    void load();
  },
);
onMounted(
  () =>
    void perform(async () => {
      if (!props.embedded)
        [runs.value, journeys.value] = await Promise.all([
          api<typeof runs.value>('/test-runs'),
          api<Journey[]>('/journeys?limit=200'),
        ]);
      await load();
    }),
);
</script>
<template>
  <section :class="{ 'embedded-inbox': embedded }">
    <div v-if="!embedded" class="page-heading">
      <div>
        <div class="eyebrow">Testing workspace</div>
        <h1>Test Inbox</h1>
        <p class="page-description">
          Review every message before it reaches your customers. Simulations stay in the sandbox.
        </p>
      </div>
      <button :disabled="busy" @click="load()">
        <AppIcon name="refresh" :size="15" />Refresh inbox
      </button>
    </div>
    <div v-else class="embedded-inbox-heading">
      <div>
        <h2>Test Inbox</h2>
        <p>Messages from this run · sandbox only</p>
      </div>
      <button :disabled="busy" @click="load()">
        <AppIcon name="refresh" :size="14" />Refresh messages
      </button>
    </div>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <div class="inbox-layout">
      <aside v-if="!embedded" class="panel inbox-filters">
        <h2>Recipients &amp; runs</h2>
        <label
          >Test run<select v-model="run" aria-label="Test run" @change="load()">
            <option value="">All runs and explicit sends</option>
            <option v-for="r in runs" :key="r.id" :value="r.id">
              {{ runTitle(r) }} · {{ r.journey_name || 'Journey' }} · {{ r.result }}
            </option>
          </select></label
        ><label>Customer ID<input v-model="customer" @keyup.enter="load()" /></label
        ><button @click="load()">Filter customer</button
        ><button v-for="c in customers" :key="c" class="list-row" @click="filterCustomer(c)">
          {{ c }}</button
        ><label
          >Journey<select v-model="journey" aria-label="Journey" @change="load()">
            <option value="">All journeys</option>
            <option v-for="j in journeys" :key="j.id" :value="j.id">{{ j.name }}</option>
          </select></label
        ><label
          >Channel<select v-model="channel" aria-label="Channel" @change="load()">
            <option value="">All channels</option>
            <option>email</option>
            <option>sms</option>
          </select></label
        ><label
          >Status<select v-model="status" aria-label="Status" @change="load()">
            <option value="">All statuses</option>
            <option
              v-for="s in [
                'SIMULATED',
                'BLOCKED',
                'SKIPPED',
                'FAILED',
                'TEST_ACCEPTED',
                'TEST_PENDING',
              ]"
              :key="s"
            >
              {{ s }}
            </option>
          </select></label
        >
      </aside>
      <section class="panel inbox-list" :aria-busy="busy">
        <div class="inbox-list-heading">
          <h2>Messages</h2>
          <span class="count-chip">{{ rows.length }}</span>
        </div>
        <p v-if="!rows.length">
          No matching test messages. Run a scenario in Test Lab or send an allowlisted test.
        </p>
        <button
          v-for="r in rows"
          :key="r.id"
          class="inbox-row"
          :class="{ selected: selected?.id === r.id }"
          @click="open(r)"
        >
          <span class="inbox-channel" aria-hidden="true"
            ><AppIcon :name="r.channel === 'sms' ? 'message' : 'email'" :size="15" /></span
          ><span class="inbox-row-body"
            ><strong>{{ r.node_label }}</strong
            ><span
              >{{ r.channel === 'sms' ? 'SMS' : 'Email' }} · {{ r.journey_name || 'Explicit test'
              }}{{ r.version ? ' · v' + r.version : '' }}</span
            ><small
              >{{ r.customer_id || 'Test recipient' }} · {{ formatDateTime(r.simulated_at) }}</small
            ></span
          ><StatusBadge :status="r.status" /></button
        ><button v-if="rows.length >= 50" @click="load(true)">Load older messages</button>
      </section>
      <section class="panel inbox-detail">
        <template v-if="selected"
          ><h2>{{ selected.message.subject || selected.node_label }}</h2>
          <div class="inbox-message-meta">
            <StatusBadge :status="selected.status" /><span>{{
              [selected.mode, selected.provider_status]
                .filter((part) => part && part !== selected!.status)
                .join(' · ')
            }}</span>
          </div>
          <p>To: {{ selected.message.to }} · {{ selected.message.preheader }}</p>
          <div class="detail-tabs">
            <button
              v-for="t in embedded
                ? ['Preview', 'Decision', 'Attempts']
                : ['Preview', 'Payload', 'Execution trace', 'Decision', 'Attempts']"
              :key="t"
              :class="{ active: tab === t }"
              :aria-pressed="tab === t"
              @click="tab = t"
            >
              {{ t }}
            </button>
          </div>
          <div v-if="!embedded" class="detail-tabs">
            <button @click="copy">Copy rendered content</button
            ><button :disabled="!selected.test_run_id || readonly || busy" @click="replay">
              Replay scenario</button
            ><button :disabled="!selected.test_run_id || busy" @click="compare">
              Compare with previous run
            </button>
          </div>
          <template v-if="tab === 'Preview'"
            ><label v-if="selected.channel === 'email'"
              >Device<select v-model="device">
                <option>desktop</option>
                <option>mobile</option>
              </select></label
            ><iframe
              v-if="selected.channel === 'email'"
              title="Test email preview"
              sandbox=""
              :srcdoc="document"
              :style="{ width: device === 'mobile' ? 'min(100%,375px)' : '100%' }"
            />
            <div v-else class="sms-phone">
              <p>{{ selected.message.text }}</p>
              <small
                >{{ selected.message.encoding }} · {{ selected.message.segments }} segments</small
              >
            </div>
            <h3>Personalization values</h3>
            <pre>{{ JSON.stringify(selected.message.personalization_values, null, 2) }}</pre>
            <p>
              Template {{ selected.message.template_id || 'inline' }} · Source node
              {{ selected.source_node || 'explicit test' }} · Trace {{ selected.trace_id }}
            </p>
            <p>Workflow snapshot {{ selected.workflow_hash || 'standalone message' }}</p></template
          >
          <pre v-else-if="tab === 'Payload'">{{ JSON.stringify(selected, null, 2) }}</pre>
          <pre v-else-if="tab === 'Execution trace'">{{
            JSON.stringify(selected.execution_trace, null, 2)
          }}</pre>
          <DecisionSummary v-else-if="tab === 'Decision'" :decision="selected.decision" />
          <AttemptHistory
            v-else-if="tab === 'Attempts'"
            :attempts="selected.attempt_history"
            :provider="selected.provider"
            :response="selected.provider_response"
          />
          <pre v-else>{{ JSON.stringify(previous, null, 2) }}</pre>
        </template>
        <EmptyState v-else class="inbox-empty" icon="inbox" :level="2" title="Your message preview">
          Select a message to inspect its rendered content, eligibility and execution trace.
        </EmptyState>
      </section>
    </div>
  </section>
</template>
<style scoped>
.inbox-row-body {
  display: grid;
  gap: 3px;
  min-width: 0;
  font-size: 12px;
  color: var(--muted);
}
.inbox-row-body strong {
  color: var(--text);
  font-size: 13px;
}
.inbox-row-body small {
  font-size: 11px;
}
.inbox-row .status-badge,
.inbox-row > :last-child {
  justify-self: end;
}
.embedded-inbox-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  gap: 12px;
}
.embedded-inbox-heading p {
  font-size: 12px;
  color: var(--muted);
  margin: 6px 0 0;
}
.embedded-inbox .inbox-layout {
  display: grid;
  grid-template-columns: minmax(180px, 0.7fr) minmax(0, 1.3fr);
  min-height: 0;
  border: 0;
  border-top: 1px solid var(--border);
  border-radius: 0 0 10px 10px;
}
.embedded-inbox .inbox-list {
  max-height: 700px;
  overflow: auto;
  border: 0;
  border-right: 1px solid var(--border);
  border-radius: 0;
  margin: 0;
  padding: 0;
}
.embedded-inbox .inbox-list-heading {
  padding: 16px;
}
.embedded-inbox .inbox-detail {
  border: 0;
  padding: 20px;
  margin: 0;
  border-radius: 0;
}
@media (max-width: 700px) {
  .embedded-inbox .inbox-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .embedded-inbox .inbox-list {
    max-height: 240px;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }
}

.inbox-layout {
  display: grid;
  grid-template-columns: 220px minmax(220px, 1fr) minmax(300px, 1.5fr);
  gap: 0;
  align-items: stretch;
  min-height: 600px;
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--surface);
}
.inbox-filters {
  background: var(--gray-99);
  display: grid;
  align-content: start;
  gap: 12px;
}
.inbox-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  justify-content: stretch;
  justify-items: start;
  gap: 12px;
  width: 100%;
  text-align: left;
  padding: 14px 16px;
  margin-bottom: 0;
  border-radius: 0;
  border: 0;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  overflow-wrap: anywhere;
}
.inbox-detail {
  min-width: 0;
}
.inbox-detail iframe {
  height: 480px;
  border: 1px solid var(--gray-90b);
  border-radius: 12px;
}
.inbox-detail pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 650px;
  overflow: auto;
}
.detail-tabs {
  flex-wrap: wrap;
}
.sms-phone {
  background: var(--page);
  border: 7px solid var(--gray-31b-line);
  border-radius: 28px;
  padding: 24px;
  max-width: 350px;
  white-space: pre-wrap;
}
@media (max-width: 1200px) {
  .inbox-layout {
    grid-template-columns: 200px 1fr;
  }
  .inbox-detail {
    grid-column: 1 / -1;
  }
}
@media (max-width: 700px) {
  .inbox-layout {
    grid-template-columns: 1fr;
  }
  /* On a phone the messages come first; the preview follows and filters close the page. */
  .inbox-list {
    order: 1;
    max-height: 420px;
    overflow: auto;
  }
  .inbox-detail {
    grid-column: auto;
    order: 2;
  }
  .inbox-filters {
    order: 3;
  }
  .inbox-layout > .panel + .panel {
    border-left: 0;
    border-top: 1px solid var(--border);
  }
}
</style>
