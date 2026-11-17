<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import { runTitle, type TestRunSummary } from '../lib/test-runs';
import EmptyState from './ui/EmptyState.vue';
import { errorMessage, formatDateTime } from '../lib/format';
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import type { TestInboxRow, TestMessage } from '../lib/test-inbox';
import MessagePreview from './MessagePreview.vue';
import PayloadSummary from './PayloadSummary.vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{
  readonly?: boolean;
  embedded?: boolean;
  testRunId?: string;
  runRevision?: number;
}>();
const rows = ref<TestInboxRow[]>([]);
const selected = ref<TestMessage>();
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
// Tabs switch between views of one message, so they read as navigation rather than a choice.
const detailTabs = computed(() =>
  props.embedded
    ? ['Preview', 'Decision', 'Attempts']
    : ['Preview', 'Payload', 'Trace', 'Decision', 'Attempts'],
);
const tabBar = ref<HTMLElement>();
const underline = ref<{ x: number; width: number; ready: boolean }>();
function placeUnderline() {
  const current = tabBar.value?.querySelector<HTMLElement>('[aria-selected="true"]');
  if (!current) {
    underline.value = undefined;
    return;
  }
  underline.value = {
    x: current.offsetLeft,
    width: current.offsetWidth,
    ready: !!underline.value,
  };
}
function stepTab(step: number) {
  const list = detailTabs.value;
  const i = list.indexOf(tab.value);
  tab.value = list[(i + step + list.length) % list.length];
  nextTick(() => tabBar.value?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus());
}
watch([tab, selected, tabBar], () => nextTick(placeUnderline));
const customers = computed(() => [
  ...new Set(rows.value.map((r) => r.customer_id).filter((c): c is string => !!c)),
]);
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
    const result = await api<TestInboxRow[]>('/test-inbox?' + params);
    if (request !== loadSequence) return;
    rows.value = more ? [...rows.value, ...result] : result;
    if (props.embedded) {
      const row = rows.value.find((row) => row.id === selected.value?.id) || rows.value[0];
      if (row) await open(row);
      else selected.value = undefined;
    }
  });
}
async function open(row: TestInboxRow) {
  const request = ++detailSequence;
  await perform(async () => {
    const detail = await api<TestMessage>('/test-messages/' + row.id);
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
async function copyValue(value: string, message: string) {
  await perform(async () => {
    await navigator.clipboard.writeText(value);
    notice.value = message;
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
// "alex-morgan" → "AM"; a single word gives its first two letters.
function initials(id: string) {
  const parts = id.split(/[-_.\s]+/).filter(Boolean);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : id.slice(0, 2)).toUpperCase();
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
        <AppSelect
          v-model="run"
          caption="Test run"
          aria-label="Test run"
          :options="[
            { value: '', label: 'All runs and explicit sends' },
            ...runs.map((r) => ({
              value: r.id,
              label: runTitle(r),
              description:
                (r.journey_name || 'Journey') +
                ' · ' +
                (r.result === 'PASS' ? 'Passed' : r.result === 'FAIL' ? 'Failed' : r.result),
              tone:
                r.result === 'PASS'
                  ? ('green' as const)
                  : r.result === 'FAIL'
                    ? ('red' as const)
                    : undefined,
            })),
          ]"
          @change="load()"
        />
        <div class="inbox-customer">
          <div class="inbox-customer-search">
            <label class="inbox-search"
              ><span class="field-label">Customer ID</span
              ><span class="inbox-search-box"
                ><AppIcon name="search" :size="15" /><input
                  v-model="customer"
                  placeholder="Search by customer ID"
                  @keyup.enter="load()" /><button
                  v-if="customer"
                  type="button"
                  class="icon-button search-clear"
                  aria-label="Clear customer"
                  @click="filterCustomer('')"
                >
                  <AppIcon name="close" :size="13" /></button></span
            ></label>
            <button
              type="button"
              class="search-submit"
              aria-label="Filter customer"
              @click="load()"
            >
              Filter
            </button>
          </div>
          <div v-if="customers.length" class="inbox-recipients">
            <span class="field-label">Recipients in these messages</span>
            <div class="recipient-chips">
              <button
                v-for="c in customers"
                :key="c"
                type="button"
                class="recipient-chip"
                :aria-pressed="customer === c"
                @click="filterCustomer(customer === c ? '' : c)"
              >
                <span class="recipient-avatar" aria-hidden="true">{{ initials(c) }}</span
                >{{ c }}
              </button>
            </div>
          </div>
        </div>
        <AppSelect
          v-model="journey"
          caption="Journey"
          aria-label="Journey"
          :options="[
            { value: '', label: 'All journeys' },
            ...journeys.map((j) => ({ value: j.id, label: j.name, icon: 'workflow' })),
          ]"
          @change="load()"
        /><SegmentedControl
          v-model="channel"
          label="Channel"
          :options="[
            { value: '', label: 'All' },
            { value: 'email', label: 'Email', icon: 'email' },
            { value: 'sms', label: 'SMS', icon: 'message' },
          ]"
          @change="load()"
        /><AppSelect
          v-model="status"
          caption="Status"
          aria-label="Status"
          :options="[
            { value: '', label: 'All statuses' },
            { value: 'SIMULATED', label: 'Simulated', tone: 'blue' },
            { value: 'TEST_ACCEPTED', label: 'Accepted', tone: 'green' },
            { value: 'TEST_PENDING', label: 'Pending', tone: 'amber' },
            {
              value: 'BLOCKED',
              label: 'Blocked',
              description: 'Consent or a limit stopped it',
              tone: 'amber',
            },
            { value: 'SKIPPED', label: 'Skipped', tone: 'neutral' },
            { value: 'FAILED', label: 'Failed', tone: 'red' },
          ]"
          @change="load()"
        />
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
              ><b class="inbox-row-channel" :class="r.channel === 'sms' ? 'sms' : 'email'">{{
                r.channel === 'sms' ? 'SMS' : 'Email'
              }}</b>
              · {{ r.journey_name || 'Explicit test'
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
          <p>
            To: {{ selected.message.to
            }}{{ selected.message.preheader ? ' · ' + selected.message.preheader : '' }}
          </p>
          <div v-if="!embedded" class="inbox-actions">
            <button type="button" @click="copy">
              <AppIcon name="copy" :size="14" />Copy rendered content</button
            ><button
              type="button"
              :disabled="!selected.test_run_id || readonly || busy"
              @click="replay"
            >
              <AppIcon name="refresh" :size="14" />Replay scenario</button
            ><button type="button" :disabled="!selected.test_run_id || busy" @click="compare">
              <AppIcon name="history" :size="14" />Compare with previous run
            </button>
          </div>
          <div ref="tabBar" class="detail-tabs" role="tablist" aria-label="Message details">
            <button
              v-for="t in detailTabs"
              :key="t"
              type="button"
              role="tab"
              :aria-selected="tab === t"
              :tabindex="tab === t ? 0 : -1"
              @click="tab = t"
              @keydown.right.prevent="stepTab(1)"
              @keydown.left.prevent="stepTab(-1)"
            >
              {{ t }}
            </button>
            <span
              v-if="underline"
              class="detail-tabs-underline"
              :class="{ sliding: underline.ready }"
              aria-hidden="true"
              :style="{ width: underline.width + 'px', transform: `translateX(${underline.x}px)` }"
            />
          </div>
          <MessagePreview v-if="tab === 'Preview'" v-model:device="device" :detail="selected" />
          <PayloadSummary v-else-if="tab === 'Payload'" :detail="selected" @copy="copyValue" />
          <pre v-else-if="tab === 'Trace'">{{
            JSON.stringify(selected.execution_trace, null, 2)
          }}</pre>
          <DecisionSummary v-else-if="tab === 'Decision'" :decision="selected.decision" />
          <AttemptHistory
            v-else-if="tab === 'Attempts'"
            :attempts="selected.attempt_history"
            :provider="selected.provider"
            :response="selected.provider_response"
            :engagement="selected.delivery_id ? selected.engagement : undefined"
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
.inbox-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 4px 0 14px;
}
.inbox-actions button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  font-size: 12px;
}
.inbox-row-body {
  display: grid;
  gap: 3px;
  min-width: 0;
  font-size: 12px;
  color: var(--muted);
}
.inbox-row-body strong {
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
}
.inbox-row-channel {
  font-weight: 600;
}
.inbox-row-channel.email {
  color: var(--violet-56b-text);
}
.inbox-row-channel.sms {
  color: var(--blue-50-text);
}
.inbox-row-body small {
  font-size: 12px;
}
/* The badge sits under the text, so the icon column stays as wide as the icon. */
.inbox-row > :last-child {
  grid-column: 2;
  justify-self: start;
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
  grid-template-columns: 248px minmax(300px, 1fr) minmax(320px, 1.15fr);
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
/* Customer filter: a search box with its action attached, then the recipients as chips. */
.inbox-customer {
  display: grid;
  gap: 12px;
}
.inbox-customer-search {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: 6px;
}
.inbox-search {
  display: grid;
  gap: 7px;
  margin: 0;
}
.inbox-search-box {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 36px;
  padding: 0 4px 0 10px;
  border: 1px solid var(--gray-91);
  border-radius: 8px;
  background: var(--surface);
  color: var(--gray-60);
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
}
.inbox-search-box:focus-within {
  border-color: var(--violet-77);
  box-shadow: 0 0 0 3px #b5a8ec26;
  color: var(--accent-text);
}
.inbox-search-box input {
  flex: 1;
  min-width: 0;
  min-height: 34px;
  padding: 0;
  border: 0;
  background: none;
  box-shadow: none;
  font-size: 13px;
  color: var(--text);
}
.inbox-search-box input:focus {
  box-shadow: none;
}
.search-clear {
  width: 24px;
  height: 24px;
  min-height: 24px;
}
.search-submit {
  min-height: 36px;
  padding: 0 12px;
  border-color: var(--violet-85);
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-weight: 600;
}
.search-submit:hover {
  border-color: var(--accent);
  background: var(--violet-93);
}
.inbox-recipients {
  display: grid;
  gap: 8px;
}
.recipient-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.recipient-chip {
  justify-content: flex-start;
  gap: 7px;
  min-height: 30px;
  padding: 3px 10px 3px 3px;
  border-radius: 999px;
  background: var(--surface);
  color: var(--gray-44b);
  font-size: 12px;
}
.recipient-chip:hover {
  border-color: var(--violet-85);
  background: var(--violet-97);
}
.recipient-chip[aria-pressed='true'] {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-weight: 600;
}
.recipient-avatar {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.recipient-chip[aria-pressed='true'] .recipient-avatar {
  background: var(--accent);
  color: var(--on-solid);
}
.inbox-filters :deep(.segmented-control > button) {
  padding: 0 8px;
  gap: 5px;
}
@media (min-width: 701px) {
  /* The list scrolls on its own and the preview stays in view, so the page does not grow
     with every message. */
  .inbox-list {
    max-height: calc(100vh - 220px);
    min-height: 560px;
    overflow: auto;
  }
  .inbox-detail {
    position: sticky;
    top: 16px;
    align-self: start;
  }
}
.inbox-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
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
.inbox-detail pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 650px;
  overflow: auto;
}
.detail-tabs {
  position: relative;
  display: flex;
  gap: 2px;
  margin: 4px 0 16px;
  flex-wrap: nowrap;
  border-bottom: 1px solid var(--border);
  overflow-x: auto;
  scrollbar-width: none;
}
.detail-tabs > button {
  flex: none;
  min-height: 40px;
  padding: 0 10px;
  border: 0;
  border-radius: 8px 8px 0 0;
  background: none;
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  transition: color 0.2s ease;
}
.detail-tabs > button:hover {
  color: var(--text);
  background: var(--gray-96);
}
.detail-tabs > button[aria-selected='true'] {
  color: var(--accent-text-strong);
  font-weight: 600;
}
.detail-tabs-underline {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 2px;
  border-radius: 2px 2px 0 0;
  background: var(--accent);
  pointer-events: none;
}
.detail-tabs-underline.sliding {
  transition:
    transform 0.4s cubic-bezier(0.22, 1, 0.36, 1),
    width 0.4s cubic-bezier(0.22, 1, 0.36, 1);
}
@media (prefers-reduced-motion: reduce) {
  .detail-tabs-underline.sliding {
    transition: none;
  }
}
/* A phone drawn around the message, so the preview reads as what the customer sees. */
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
