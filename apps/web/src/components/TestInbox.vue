<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import { runTitle, type TestRunSummary } from '../lib/test-runs';
import EmptyState from './ui/EmptyState.vue';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import type { DeliveryEngagement } from '../lib/engagement';
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
  engagement?: DeliveryEngagement[];
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
// Tabs switch between views of one message, so they read as navigation rather than a choice.
const detailTabs = computed(() =>
  props.embedded
    ? ['Preview', 'Decision', 'Attempts']
    : ['Preview', 'Payload', 'Trace', 'Decision', 'Attempts'],
);
// Desktop email renders at a typical client width and is scaled down to fit the column.
const MAIL_WIDTH = 640;
const MAIL_HEIGHT = 760;
const mailBody = ref<HTMLElement>();
const mailDialog = ref<HTMLDialogElement>();
const mailOpen = ref(false);
function openMail() {
  mailOpen.value = true;
  mailDialog.value?.showModal();
}
const mailScale = ref(1);
const mailHeight = computed(() => Math.round(MAIL_HEIGHT * mailScale.value));
const mailObserver =
  typeof ResizeObserver === 'undefined'
    ? undefined
    : new ResizeObserver(([entry]) => {
        mailScale.value = Math.min(1, entry.contentRect.width / MAIL_WIDTH);
      });
watch(mailBody, (element, old) => {
  if (old) mailObserver?.unobserve(old);
  if (element) mailObserver?.observe(element);
});
// The phone shows the email at a real phone's width (390 px) scaled to its screen.
const PHONE_WIDTH = 390;
const phoneScreen = ref<HTMLElement>();
const phoneScale = ref(0.77);
const phoneObserver =
  typeof ResizeObserver === 'undefined'
    ? undefined
    : new ResizeObserver(([entry]) => {
        if (entry.contentRect.width) phoneScale.value = entry.contentRect.width / PHONE_WIDTH;
      });
watch(phoneScreen, (element, old) => {
  if (old) phoneObserver?.unobserve(old);
  if (element) phoneObserver?.observe(element);
});
onBeforeUnmount(() => {
  mailObserver?.disconnect();
  phoneObserver?.disconnect();
});
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
const escape = (s: string) =>
  s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const document = computed(
  () =>
    `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none';style-src 'unsafe-inline';img-src data:;form-action 'none';base-uri 'none'"><meta name="viewport" content="width=device-width"><style>body{margin:0;font:16px/1.6 Arial}body>:not(table){margin-left:20px;margin-right:20px}img{max-width:100%}</style></head><body>${selected.value?.message.html || '<pre>' + escape(selected.value?.message.text || '') + '</pre>'}</body></html>`,
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
const personalization = computed(() =>
  Object.entries(selected.value?.message.personalization_values || {}),
);
const payloadIds = computed(() => [
  { label: 'Trace', icon: 'history', value: selected.value?.trace_id, empty: 'Not recorded' },
  {
    label: 'Workflow snapshot',
    icon: 'lock',
    value: selected.value?.workflow_hash,
    empty: 'Standalone message',
  },
]);
// Long IDs stay readable at a glance; the full value is in the tooltip and copy button.
const shortId = (value: string) => (value.length > 16 ? value.slice(0, 12) + '…' : value);
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
// SMS text split so links can be drawn as a phone draws them.
const smsParts = computed(() =>
  (selected.value?.message.text || '')
    .split(/(https?:\/\/\S+)/)
    .filter(Boolean)
    .map((text) => ({ text, link: /^https?:\/\//.test(text) })),
);
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
          <template v-if="tab === 'Preview'"
            ><SegmentedControl
              v-if="selected.channel === 'email'"
              v-model="device"
              class="device-segments"
              label="Device"
              hide-label
              :options="[
                { value: 'desktop', label: 'Desktop', icon: 'monitor' },
                { value: 'mobile', label: 'Mobile' },
              ]"
            /><!-- A mail client window, so the desktop view reads as an opened email. -->
            <div v-if="selected.channel === 'email' && device === 'desktop'" class="mail-stage">
              <div class="mail-window">
                <div class="mail-titlebar">
                  <span aria-hidden="true"><i /><i /><i /></span>
                  <button type="button" class="mail-expand" @click="openMail">
                    <AppIcon name="maximize" :size="13" />Open full size
                  </button>
                </div>
                <header class="mail-header">
                  <span class="mail-avatar" aria-hidden="true"
                    ><AppIcon name="email" :size="16"
                  /></span>
                  <div>
                    <strong>{{ selected.message.subject || 'No subject' }}</strong>
                    <span
                      >To {{ selected.message.to || selected.customer_id }} ·
                      {{ formatDateTime(selected.simulated_at) }}</span
                    >
                    <small v-if="selected.message.preheader">{{
                      selected.message.preheader
                    }}</small>
                  </div>
                </header>
                <div ref="mailBody" class="mail-body" :style="{ height: mailHeight + 'px' }">
                  <iframe
                    title="Test email preview"
                    sandbox=""
                    :srcdoc="document"
                    :style="{ transform: `scale(${mailScale})` }"
                  />
                </div>
              </div>
              <dialog
                ref="mailDialog"
                class="mail-dialog"
                aria-labelledby="mail-dialog-title"
                @close="mailOpen = false"
                @click="$event.target === mailDialog && mailDialog?.close()"
              >
                <header>
                  <h2 id="mail-dialog-title">
                    {{ selected.message.subject || 'Test email' }}
                  </h2>
                  <button
                    type="button"
                    class="icon-button"
                    aria-label="Close full size preview"
                    @click="mailDialog?.close()"
                  >
                    <AppIcon name="close" :size="16" />
                  </button>
                </header>
                <iframe
                  v-if="mailOpen"
                  title="Test email preview, full size"
                  sandbox=""
                  :srcdoc="document"
                />
              </dialog>
            </div>
            <!-- The preview shows what the customer sees; technical details are under Payload. -->
            <div v-else class="inbox-phone-preview">
              <div
                class="phone"
                :class="selected.channel === 'email' ? 'phone-email' : 'phone-sms'"
              >
                <div class="phone-status" aria-hidden="true">
                  <span>9:41</span><span class="phone-island" /><span class="phone-signal"
                    ><i /><i /><i /><i
                  /></span>
                </div>
                <div v-if="selected.channel === 'email'" ref="phoneScreen" class="phone-screen">
                  <iframe
                    title="Test email preview"
                    sandbox=""
                    :srcdoc="document"
                    :style="{
                      transform: `scale(${phoneScale})`,
                      height: `calc(100% / ${phoneScale})`,
                    }"
                  />
                </div>
                <template v-else>
                  <div class="phone-thread-head">
                    <span class="phone-avatar" aria-hidden="true"
                      ><AppIcon name="message" :size="16"
                    /></span>
                    <strong>{{ selected.journey_name || 'Test message' }}</strong>
                    <small>Text message</small>
                  </div>
                  <div class="phone-thread">
                    <small class="phone-time">{{ formatDateTime(selected.simulated_at) }}</small>
                    <p class="sms-phone">
                      <template v-for="(part, i) in smsParts" :key="i"
                        ><span v-if="part.link" class="sms-link">{{ part.text }}</span
                        ><template v-else>{{ part.text }}</template></template
                      >
                    </p>
                  </div>
                  <div class="phone-compose" aria-hidden="true"><span>Text message</span></div>
                </template>
                <span class="phone-home" aria-hidden="true" />
              </div>
              <small v-if="selected.channel === 'sms'" class="phone-meta"
                >{{ (selected.message.text || '').length }} characters ·
                {{ selected.message.encoding }} ·
                {{ plural(selected.message.segments ?? 0, 'segment') }}</small
              >
            </div></template
          >
          <template v-else-if="tab === 'Payload'">
            <dl class="payload-summary">
              <div>
                <dt><AppIcon name="templates" :size="14" />Template</dt>
                <dd>{{ selected.message.template_id || 'Inline content' }}</dd>
              </div>
              <div>
                <dt><AppIcon name="workflow" :size="14" />Source step</dt>
                <dd>{{ selected.source_node || 'Explicit test' }}</dd>
              </div>
              <div v-for="id in payloadIds" :key="id.label" class="payload-id">
                <dt><AppIcon :name="id.icon" :size="14" />{{ id.label }}</dt>
                <dd>
                  <code :title="id.value">{{ id.value ? shortId(id.value) : id.empty }}</code>
                  <button
                    v-if="id.value"
                    type="button"
                    class="icon-button payload-copy"
                    :aria-label="`Copy ${id.label.toLowerCase()} ID`"
                    @click="copyValue(id.value, `${id.label} ID copied`)"
                  >
                    <AppIcon name="copy" :size="13" />
                  </button>
                </dd>
              </div>
            </dl>
            <section class="payload-section" aria-labelledby="personalization-heading">
              <h3 id="personalization-heading">
                Personalization values
                <span v-if="personalization.length" class="payload-count">{{
                  personalization.length
                }}</span>
              </h3>
              <ul v-if="personalization.length" class="token-list">
                <li v-for="[key, value] in personalization" :key="key">
                  <code class="token-key">{{ key }}</code>
                  <AppIcon name="arrow" :size="14" class="token-arrow" />
                  <span class="token-value">{{ value || 'Empty' }}</span>
                </li>
              </ul>
              <p v-else class="help-text">This message uses no personalization.</p>
            </section>
            <details class="raw-payload">
              <summary>
                <AppIcon name="file" :size="14" />Raw payload
                <span>JSON</span>
                <AppIcon name="down" :size="14" class="raw-chevron" />
              </summary>
              <pre>{{ JSON.stringify(selected, null, 2) }}</pre>
            </details>
          </template>
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
.inbox-row-body small {
  font-size: 12px;
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
.mail-stage {
  padding: 16px 12px;
  border-radius: 12px;
  background: var(--gray-96);
}
.mail-window {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  box-shadow:
    0 1px 2px #10182814,
    0 12px 32px -12px #10182833;
}
.mail-titlebar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--gray-99);
}
.mail-titlebar > span {
  display: flex;
  gap: 6px;
}
.mail-expand {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 26px;
  margin: -4px -4px -4px auto;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--muted);
  font-size: 12px;
}
.mail-expand:hover {
  background: var(--gray-96);
  color: var(--text);
}
.mail-dialog {
  width: min(760px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  color: var(--text);
}
.mail-dialog::backdrop {
  background: #0f111599;
}
.mail-dialog header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 12px 12px 20px;
  border-bottom: 1px solid var(--border);
}
.mail-dialog h2 {
  flex: 1;
  margin: 0;
  font-size: 15px;
}
.mail-dialog iframe {
  display: block;
  width: 100%;
  height: min(78vh, 900px);
  border: 0;
  background: var(--phone-screen);
}
.mail-titlebar i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--edge-muted);
}
.mail-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
}
.mail-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
}
.mail-header div {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.mail-header strong {
  font-size: 14px;
  color: var(--text);
  overflow-wrap: anywhere;
}
.mail-header span,
.mail-header small {
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.mail-body {
  position: relative;
  overflow: hidden;
  background: var(--phone-screen);
}
.mail-body iframe {
  position: absolute;
  top: 0;
  left: 0;
  width: 640px;
  height: 760px;
  border: 0;
  transform-origin: 0 0;
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
.inbox-phone-preview {
  display: grid;
  justify-items: center;
  gap: 12px;
  padding: 24px 12px;
  border-radius: 12px;
  background: var(--gray-96);
}
.phone {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(100%, 320px);
  height: 600px;
  padding: 10px 12px 14px;
  border: 9px solid var(--gray-25);
  border-radius: 46px;
  background: var(--phone-screen);
  box-shadow:
    0 0 0 1px var(--gray-74),
    0 24px 48px -16px #17213b4d;
  overflow: hidden;
}
.phone-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 14px 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}
.phone-island {
  width: 86px;
  height: 24px;
  border-radius: 999px;
  background: var(--gray-25);
}
.phone-signal {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 11px;
}
.phone-signal i {
  width: 3px;
  border-radius: 1px;
  background: var(--text);
}
.phone-signal i:nth-child(1) {
  height: 4px;
}
.phone-signal i:nth-child(2) {
  height: 6px;
}
.phone-signal i:nth-child(3) {
  height: 8px;
}
.phone-signal i:nth-child(4) {
  height: 11px;
}
.phone-thread-head {
  display: grid;
  justify-items: center;
  gap: 3px;
  padding: 6px 0 12px;
  border-bottom: 1px solid var(--border);
}
.phone-avatar {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: linear-gradient(160deg, var(--gray-74), var(--gray-55));
  color: var(--on-solid);
}
.phone-thread-head strong {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--text);
}
.phone-thread-head small {
  font-size: 11px;
  color: var(--muted);
}
.phone-thread {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 14px 4px;
  overflow: auto;
}
.phone-time {
  align-self: center;
  font-size: 11px;
  color: var(--muted);
}
.sms-phone {
  max-width: 82%;
  margin: 0;
  padding: 9px 13px;
  border-radius: 18px 18px 18px 5px;
  background: var(--phone-bubble);
  color: var(--text);
  font-size: 14px;
  line-height: 1.4;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.sms-link {
  color: var(--phone-link);
  text-decoration: underline;
}
.phone-compose {
  display: flex;
  align-items: center;
  min-height: 34px;
  margin: 0 4px 10px;
  padding: 0 14px;
  border: 1px solid var(--border);
  border-radius: 999px;
  font-size: 13px;
  color: var(--gray-60);
}
.phone-home {
  align-self: center;
  width: 110px;
  height: 4px;
  border-radius: 999px;
  background: var(--text);
  opacity: 0.8;
}
.phone-screen {
  position: relative;
  flex: 1;
  margin: 0 -12px 8px;
  overflow: hidden;
}
.phone-screen iframe {
  position: absolute;
  top: 0;
  left: 0;
  width: 390px;
  border: 0;
  border-radius: 0;
  transform-origin: 0 0;
}
.phone-meta {
  font-size: 12px;
  color: var(--muted);
}
.payload-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 0 0 20px;
}
.payload-summary > div {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 11px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--gray-99);
}
.payload-summary dt {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}
.payload-summary dt :deep(svg) {
  color: var(--accent-text);
}
.payload-summary dd {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  overflow-wrap: anywhere;
}
.payload-id code {
  min-width: 0;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.payload-copy {
  flex: none;
  width: 28px;
  height: 28px;
  margin: -4px -4px -4px auto;
}
.payload-section h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 14px;
}
.payload-count {
  min-width: 20px;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-size: 12px;
  text-align: center;
}
.token-list {
  display: grid;
  gap: 8px;
  margin: 0 0 20px;
  padding: 0;
  list-style: none;
}
.token-list li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
.token-key {
  justify-self: start;
  max-width: 100%;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.token-arrow {
  color: var(--muted);
}
.token-value {
  margin-left: auto;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  overflow-wrap: anywhere;
}
.raw-payload {
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}
.raw-payload summary {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  cursor: pointer;
  list-style: none;
}
.raw-payload summary::-webkit-details-marker {
  display: none;
}
.raw-payload summary:hover {
  background: var(--gray-99);
}
.raw-payload summary span {
  padding: 1px 6px;
  border-radius: 5px;
  background: var(--gray-96);
  color: var(--muted);
  font-size: 11px;
  font-weight: 500;
}
.raw-chevron {
  margin-left: auto;
  color: var(--muted);
  transition: transform 0.2s ease;
}
.raw-payload[open] .raw-chevron {
  transform: rotate(180deg);
}
.raw-payload pre {
  margin: 0;
  border-top: 1px solid var(--border);
  border-radius: 0;
}
@media (max-width: 520px) {
  .payload-summary {
    grid-template-columns: 1fr;
  }
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
