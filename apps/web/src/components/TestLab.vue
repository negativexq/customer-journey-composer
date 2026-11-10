<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { api, type Journey } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import CustomerProfileFields from './ui/CustomerProfileFields.vue';
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
const mode = ref(props.nodeId ? 'single_node' : 'full');
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
const versions = ref<{ version: number }[]>([]);
const outcome = ref('COMPLETED');
const expectedMessages = ref('any');
const entryName = computed({
  get() {
    try {
      return JSON.parse(entry.value).event_name || '';
    } catch {
      return '';
    }
  },
  set(value: string) {
    try {
      entry.value = JSON.stringify({ ...JSON.parse(entry.value), event_name: value }, null, 2);
    } catch {
      error.value = 'Correct the advanced entry-event JSON first.';
    }
  },
});
const eventRows = computed(() => {
  try {
    return JSON.parse(events.value) as {
      after_seconds: number;
      event: { event_name: string; event_id?: string };
    }[];
  } catch {
    return [];
  }
});
function addEvent() {
  const rows = [
    ...eventRows.value,
    {
      after_seconds: 3600,
      event: { event_name: 'purchase_completed', event_id: crypto.randomUUID() },
    },
  ];
  events.value = JSON.stringify(rows, null, 2);
}
function changeEvent(index: number, key: string, value: string | number) {
  const rows = JSON.parse(events.value);
  if (key === 'hours') rows[index].after_seconds = Math.round(Number(value) * 3600);
  else rows[index].event.event_name = String(value);
  events.value = JSON.stringify(rows, null, 2);
}
function removeEvent(index: number) {
  events.value = JSON.stringify(
    eventRows.value.filter((_, i) => i !== index),
    null,
    2,
  );
}
watch([outcome, expectedMessages], () => {
  const checks: Record<string, unknown>[] = [
    { type: 'state', expected: outcome.value },
    { type: 'no_duplicate_delivery', expected: true },
  ];
  if (expectedMessages.value === 'none') checks.push({ type: 'no_message' });
  if (['email', 'both'].includes(expectedMessages.value))
    checks.push({ type: 'message_sent', channel: 'email' });
  if (['sms', 'both'].includes(expectedMessages.value))
    checks.push({ type: 'message_sent', channel: 'sms' });
  assertions.value = JSON.stringify(checks, null, 2);
});
async function loadVersions() {
  if (!journey.value) {
    versions.value = [];
    return;
  }
  try {
    versions.value = await api('/journeys/' + journey.value + '/versions');
  } catch (cause) {
    error.value = String(cause);
  }
}
watch(journey, () => void loadVersions());
const time = (at: string) =>
  new Date(at).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'UTC',
  }) + ' UTC';
function decision(output: unknown) {
  const o = output as Record<string, unknown>;
  if (o.condition !== undefined)
    return o.condition ? 'Condition matched' : 'Condition did not match';
  if (o.exit_reason) return 'Exit: ' + String(o.exit_reason).replaceAll('_', ' ');
  if (o.wait_until) return 'Waiting until ' + time(String(o.wait_until));
  return o.port ? 'Continued via ' + String(o.port) : 'Step recorded';
}
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
      await loadVersions();
    }),
);
</script>
<template>
  <section class="test-lab">
    <div class="page-heading">
      <div>
        <h1>Test Lab</h1>
        <p class="page-description">
          Try a customer scenario and fast-forward time before going live.
        </p>
      </div>
      <span class="sandbox-badge"><AppIcon name="flask" :size="15" />Isolated sandbox</span>
    </div>
    <p v-if="error" class="inline-alert error-alert" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-message" role="status">{{ notice }}</p>
    <div class="test-lab-layout">
      <aside class="surface scenario-panel">
        <div class="surface-heading">
          <h2>Scenario setup</h2>
          <AppIcon name="settings" :size="16" />
        </div>
        <div class="scenario-form">
          <label v-if="!journeyId"
            >Journey<select v-model="journey" @change="refresh">
              <option value="">Select a journey</option>
              <option v-for="j in journeys" :key="j.id" :value="j.id">{{ j.name }}</option>
            </select></label
          ><label
            >Saved scenario<select v-model="savedScenario">
              <option value="">Create a new scenario</option>
              <option
                v-for="s in scenarios.filter((s) => !journey || s.journey_id === journey)"
                :key="s.id"
                :value="s.id"
              >
                {{ s.name }}
              </option>
            </select></label
          ><label
            >Workflow version<select v-model.number="version">
              <option :value="0">Current draft</option>
              <option v-for="v in versions" :key="v.version" :value="v.version">
                Published v{{ v.version }}
              </option>
            </select></label
          ><label v-if="savedScenario" class="checkbox-label"
            ><input v-model="currentDraft" type="checkbox" />Use current draft for regression</label
          >
          <div v-if="savedScenario" class="scenario-snapshot">
            <AppIcon name="history" :size="17" />
            <p>Uses the saved workflow and customer snapshot. Replay keeps the original inputs.</p>
          </div>
          <template v-else
            ><label>Scenario name<input v-model="name" maxlength="160" /></label>
            <div class="form-section-title">Test customer</div>
            <label
              >Profile<select v-model="profile" @change="preset">
                <option>Custom Test Customer</option>
                <option>Existing Customer</option>
                <option>New Customer</option>
                <option>Returning Customer</option>
                <option>Inactive Customer</option>
              </select></label
            ><label v-if="profile === 'Existing Customer'"
              >Source customer ID<input v-model="customerID" placeholder="Customer ID" /></label
            ><CustomerProfileFields v-model="customer" />
            <div class="form-section-title">Entry &amp; events</div>
            <label>Entry event name<input v-model="entryName" placeholder="cart_updated" /></label>
            <div v-for="(e, i) in eventRows" :key="i" class="scenario-event">
              <label
                >Event name<input
                  :value="e.event.event_name"
                  @input="changeEvent(i, 'name', ($event.target as HTMLInputElement).value)"
              /></label>
              <div>
                <label
                  >After (hours)<input
                    :value="e.after_seconds / 3600"
                    type="number"
                    min="0"
                    step="0.25"
                    @input="
                      changeEvent(i, 'hours', Number(($event.target as HTMLInputElement).value))
                    " /></label
                ><button class="icon-button" aria-label="Remove event" @click="removeEvent(i)">
                  <AppIcon name="close" :size="15" />
                </button>
              </div>
            </div>
            <button class="add-event-button" @click="addEvent">
              <AppIcon name="plus" :size="15" />Add event
            </button>
            <div class="form-section-title">Expected results</div>
            <label
              >Expected outcome<select v-model="outcome">
                <option value="COMPLETED">Journey completes</option>
                <option value="EXITED">Customer exits</option>
                <option value="FAILED">Journey fails</option>
                <option value="WAITING">Still waiting</option>
              </select></label
            ><label
              >Expected messages<select v-model="expectedMessages">
                <option value="any">Use default assertions</option>
                <option value="both">Email and SMS</option>
                <option value="email">Email is generated</option>
                <option value="sms">SMS is generated</option>
                <option value="none">No messages</option>
              </select></label
            >
            <details class="advanced-options">
              <summary>Advanced scenario options <AppIcon name="down" :size="13" /></summary>
              <div class="editor-form">
                <label
                  >Start time (UTC)<input v-model="start" type="datetime-local" step="1" /></label
                ><label
                  >Test mode<select v-model="mode">
                    <option value="full">Full journey / event sequence</option>
                    <option value="single_node">Single node</option>
                  </select></label
                ><label v-if="mode === 'single_node'">Node ID<input v-model="node" /></label
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
                ><label
                  >Customer overrides (JSON)<textarea
                    v-model="customer"
                    rows="8"
                    spellcheck="false"
                  /></label
                ><label
                  >Entry event (JSON)<textarea v-model="entry" rows="5" spellcheck="false" /></label
                ><label
                  >Event sequence (JSON; after_seconds, event)<textarea
                    v-model="events"
                    rows="5"
                    spellcheck="false"
                  /></label
                ><label
                  >Attribute changes (JSON; after_seconds, attributes)<textarea
                    v-model="changes"
                    rows="5"
                    spellcheck="false"
                  /></label
                ><label
                  >Assertions (JSON)<textarea v-model="assertions" rows="5" spellcheck="false" />
                </label>
              </div></details
          ></template>
          <div class="scenario-actions">
            <button
              class="primary"
              :disabled="busy || readonly || (!journey && !savedScenario)"
              @click="create"
            >
              <AppIcon name="play" :size="15" />Create test run</button
            ><button :disabled="busy || readonly" @click="saveScenario">
              <AppIcon name="file" :size="15" />Save scenario</button
            ><button class="text-button" :disabled="busy" @click="preview">
              Preview journey path <AppIcon name="arrow" :size="14" />
            </button>
          </div>
        </div>
      </aside>
      <div class="test-results-column">
        <section v-if="run" class="surface virtual-clock">
          <div>
            <span class="clock-icon"><AppIcon name="clock" :size="20" /></span>
            <div>
              <small>VIRTUAL CLOCK</small><strong>{{ time(run.state.now) }}</strong>
            </div>
            <StatusBadge :status="run.state.paused ? 'PAUSED' : run.state.status" />
          </div>
          <div class="clock-controls">
            <button class="primary" :disabled="busy || readonly" @click="advance('run')">
              <AppIcon name="play" :size="14" />Run</button
            ><button :disabled="busy || readonly" @click="advance('step')">Step</button
            ><button :disabled="busy || readonly" @click="advance('advance', 3600)">
              Advance 1h</button
            ><button :disabled="busy || readonly" @click="advance('advance', 86400)">
              Advance 1d</button
            ><button :disabled="busy || readonly" @click="advance('until_next')">
              Run until next event</button
            ><button :disabled="busy || readonly" @click="advance('complete')">
              Run to completion</button
            ><button
              v-if="!run.state.paused"
              :disabled="busy || readonly"
              @click="advance('pause')"
            >
              <AppIcon name="pause" :size="14" />Pause</button
            ><button v-else :disabled="busy || readonly" @click="advance('resume')">Resume</button
            ><button class="text-button" :disabled="busy || readonly" @click="replay">
              <AppIcon name="refresh" :size="13" />Reset / Replay
            </button>
          </div>
        </section>
        <section class="surface execution-preview">
          <div class="surface-heading">
            <div>
              <h2>{{ path ? 'Journey path preview' : 'Execution timeline' }}</h2>
              <p>
                {{
                  state
                    ? 'Follow each decision at its simulated time.'
                    : 'Your test results will appear here.'
                }}
              </p>
            </div>
            <StatusBadge v-if="state" :status="state.result" />
          </div>
          <div v-if="!state" class="test-start-empty">
            <span class="empty-icon"><AppIcon name="flask" :size="32" /></span>
            <h3>Ready for a test drive?</h3>
            <p>
              Choose a scenario, create a test run, then use the virtual clock to explore its path.
            </p>
            <div>
              <span><AppIcon name="shield" :size="13" />No live profile changes</span
              ><span><AppIcon name="email" :size="13" />No provider calls</span>
            </div>
          </div>
          <template v-else
            ><div class="run-result-heading">
              <span
                >Result <strong data-testid="test-result">{{ state.result }}</strong></span
              ><span>{{ state.status }} {{ state.paused ? '· paused' : '' }}</span
              ><span v-if="state.exit_reason">{{ state.exit_reason.replaceAll('_', ' ') }}</span>
            </div>
            <div class="assertion-list">
              <span v-for="(a, i) in state.assertions" :key="i" :class="{ passed: a.pass }"
                ><AppIcon :name="a.pass ? 'check' : 'close'" :size="13" />{{
                  a.description.replaceAll('_', ' ')
                }}
                <small>{{ a.actual }}</small></span
              >
            </div>
            <div class="execution-timeline">
              <div
                v-for="(s, i) in state.steps"
                :key="i"
                class="timeline-step"
                :class="s.state.toLowerCase()"
              >
                <span class="timeline-node"
                  ><AppIcon
                    :name="
                      s.state === 'WAITING' ? 'clock' : s.state === 'FAILED' ? 'close' : 'check'
                    "
                    :size="15"
                /></span>
                <div class="timeline-step-content">
                  <div>
                    <strong>{{ s.label }}</strong
                    ><StatusBadge :status="s.state" />
                  </div>
                  <p>{{ decision(s.output) }}</p>
                  <time
                    >{{ time(s.started_at)
                    }}<span v-if="s.deadline"> · deadline {{ time(s.deadline) }}</span></time
                  >
                </div>
              </div>
            </div>
            <div class="sandbox-output">
              <h3>
                Sandbox messages <span class="count-label">{{ state.messages.length }}</span>
              </h3>
              <p v-if="!state.messages.length" class="muted">No messages produced.</p>
              <div v-for="m in state.messages" :key="m.id" class="sandbox-message-row">
                <span class="channel-icon" :class="m.channel"
                  ><AppIcon :name="m.channel === 'email' ? 'email' : 'message'" :size="16"
                /></span>
                <div>
                  <strong>{{ m.channel === 'email' ? 'Email' : 'SMS' }} message</strong
                  ><small>{{ time(m.scheduled_at) }}</small>
                </div>
                <StatusBadge :status="m.status" />
              </div>
              <p class="card-footnote">
                Open Test Inbox to inspect rendered content and delivery decisions.
              </p>
            </div>
            <details class="trace-details">
              <summary>
                <AppIcon name="file" :size="14" />Execution trace ({{ state.trace.length }})
              </summary>
              <p v-for="(t, i) in state.trace" :key="i">
                {{ time(t.at) }} · {{ t.kind }} · {{ t.message }}
              </p>
            </details></template
          >
        </section>
        <section class="surface recent-test-runs">
          <div class="surface-heading">
            <h2>Recent test runs</h2>
            <span class="count-label">{{ runs.length }}</span>
          </div>
          <div v-if="!runs.length" class="small-empty">
            Saved runs and their results will appear here.
          </div>
          <button v-for="r in runs" :key="r.id" class="test-run-row" @click="open(r.id)">
            <span class="test-run-symbol"><AppIcon name="flask" :size="15" /></span
            ><span
              ><strong>{{ r.id.slice(0, 8) }}</strong
              ><small>{{ new Date(r.created_at).toLocaleString() }}</small></span
            ><StatusBadge :status="r.result" /><AppIcon name="arrow" :size="14" />
          </button>
        </section>
      </div>
    </div>
  </section>
</template>
