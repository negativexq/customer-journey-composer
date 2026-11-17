<script setup lang="ts">
import EmptyState from './ui/EmptyState.vue';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import StatusBadge from './ui/StatusBadge.vue';
import AppIcon from './ui/AppIcon.vue';
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { computed, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import type { InspectionKind } from '../lib/navigation';
import type { DataEntry } from '../lib/data-catalog';

type EventRow = {
  event_id: string;
  customer_id: string;
  event_name: string;
  occurred_at: string;
  received_at: string;
  status: 'ACCEPTED' | 'LATE';
  duplicate_count: number;
};
type Ingestion = { event_id: string; status: string; executions: string[] };

const emit = defineEmits<{ inspect: [{ kind: InspectionKind; id: string }] }>();
defineProps<{ readonly?: boolean }>();

const pageSize = 100;
const events = ref<EventRow[]>([]);
const loading = ref(true);
const loadingMore = ref(false);
const more = ref(false);
const error = ref('');
const search = ref('');
const nameFilter = ref('');
const statusFilter = ref('');

// Suggestions for the form: names the data catalog knows plus names already seen,
// and recent customer IDs. Typing anything else stays allowed.
const catalogNames = ref<string[]>([]);
const customerIDs = ref<string[]>([]);

const customer = ref('');
const name = ref('cart_updated');
const eventID = ref(crypto.randomUUID() as string);
const occurred = ref(localNow());
const properties = ref(
  JSON.stringify({ cart_id: 'cart_demo', amount: 2450, currency: 'TRY' }, null, 2),
);
const sending = ref(false);
const result = ref<Ingestion>();
const fresh = ref('');

function localNow() {
  const d = new Date();
  d.setMilliseconds(0);
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 19);
}

async function load() {
  error.value = '';
  loading.value = true;
  try {
    const rows = await api<EventRow[]>(`/events?limit=${pageSize}`);
    events.value = rows;
    more.value = rows.length === pageSize;
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    loading.value = false;
  }
}
async function loadMore() {
  const last = events.value.at(-1);
  if (!last) return;
  loadingMore.value = true;
  try {
    const rows = await api<EventRow[]>(
      `/events?limit=${pageSize}&before=${encodeURIComponent(last.received_at)}`,
    );
    const seen = new Set(events.value.map((e) => e.event_id));
    events.value = [...events.value, ...rows.filter((e) => !seen.has(e.event_id))];
    more.value = rows.length === pageSize;
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    loadingMore.value = false;
  }
}
async function loadSuggestions() {
  const [catalog, customers] = await Promise.allSettled([
    api<DataEntry[]>('/data-catalog'),
    api<{ customer_id: string }[]>('/customers?limit=50'),
  ]);
  if (catalog.status === 'fulfilled')
    catalogNames.value = catalog.value
      .filter((e) => e.kind === 'event' && e.enabled && e.event_name)
      .map((e) => e.event_name as string);
  if (customers.status === 'fulfilled')
    customerIDs.value = customers.value.map((c) => c.customer_id);
}

const eventNames = computed(() =>
  [...new Set([...catalogNames.value, ...events.value.map((e) => e.event_name)])].sort(),
);
const nameOptions = computed(() => [
  { value: '', label: 'All events' },
  ...eventNames.value.map((n) => ({ value: n, label: n })),
]);
const statusOptions = [
  { value: '', label: 'All' },
  { value: 'ACCEPTED', label: 'Accepted' },
  { value: 'LATE', label: 'Late' },
];
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  return events.value.filter(
    (e) =>
      (!nameFilter.value || e.event_name === nameFilter.value) &&
      (!statusFilter.value || e.status === statusFilter.value) &&
      (!q || e.customer_id.toLowerCase().includes(q) || e.event_name.toLowerCase().includes(q)),
  );
});
const filtering = computed(() => !!(search.value.trim() || nameFilter.value || statusFilter.value));

// Day headings keep a long feed scannable: "Today", "Yesterday", then dates.
function dayLabel(value: string) {
  const date = new Date(value);
  const start = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((start(new Date()) - start(date)) / 86400000);
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  return date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
}
const groups = computed(() => {
  const out: { day: string; rows: EventRow[] }[] = [];
  for (const row of filtered.value) {
    const day = dayLabel(row.received_at);
    if (out.at(-1)?.day !== day) out.push({ day, rows: [] });
    out.at(-1)!.rows.push(row);
  }
  return out;
});
function clock(value: string) {
  return new Date(value).toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  });
}
function clearFilters() {
  search.value = '';
  nameFilter.value = '';
  statusFilter.value = '';
}
// The message provider reports deliveries, opens and clicks as events of the customer.
const deliveryReports = new Set([
  'message_delivered',
  'message_opened',
  'message_clicked',
  'message_bounced',
  'message_unsubscribed',
]);
function isDeliveryReport(name: string) {
  return deliveryReports.has(name);
}
function inspect(id: string) {
  emit('inspect', { kind: 'event', id });
}

// Properties are validated as you type, so a typo never reaches the server.
const parsed = computed<{ value?: Record<string, unknown>; error?: string }>(() => {
  const text = properties.value.trim();
  if (!text) return { value: {} };
  try {
    const value = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value))
      return { error: 'Properties must be a JSON object, like {"amount": 2450}.' };
    return { value };
  } catch (cause) {
    return { error: `Not valid JSON yet: ${(cause as Error).message}` };
  }
});
function formatProperties() {
  if (parsed.value.value) properties.value = JSON.stringify(parsed.value.value, null, 2);
}
const occurredDate = computed(() => new Date(occurred.value));
const timing = computed(() => {
  const at = occurredDate.value.getTime();
  if (Number.isNaN(at)) return { tone: 'error', text: 'Choose when the event happened.' };
  const diff = at - Date.now();
  if (diff > 5 * 60000)
    return { tone: 'error', text: 'More than 5 minutes in the future; the API rejects it.' };
  if (-diff > 24 * 3600000)
    return { tone: 'warn', text: 'Older than 24 hours: it is stored as Late and starts nothing.' };
  return { tone: '', text: '' };
});
const canSend = computed(
  () =>
    !sending.value &&
    !!customer.value.trim() &&
    !!name.value.trim() &&
    !!eventID.value.trim() &&
    !parsed.value.error &&
    timing.value.tone !== 'error',
);

async function send() {
  if (!canSend.value) return;
  error.value = '';
  sending.value = true;
  try {
    result.value = await api<Ingestion>('/events', 'POST', {
      event_id: eventID.value.trim(),
      customer_id: customer.value.trim(),
      event_name: name.value.trim(),
      occurred_at: occurredDate.value.toISOString(),
      properties: parsed.value.value,
    });
    fresh.value = result.value.event_id;
    await load();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    sending.value = false;
  }
}
// A new ID makes the next send a separate event; keeping it shows deduplication.
function newEvent() {
  eventID.value = crypto.randomUUID();
  occurred.value = localNow();
  result.value = undefined;
}
const outcome = computed(() => {
  const r = result.value;
  if (!r) return undefined;
  const started = r.executions.length;
  if (r.status === 'DUPLICATE')
    return {
      tone: 'amber',
      icon: 'copy',
      title: 'Duplicate, ignored',
      text: 'This event ID was already received, so nothing ran twice.',
    };
  if (r.status === 'LATE')
    return {
      tone: 'amber',
      icon: 'clock',
      title: 'Stored as late',
      text: 'It happened over 24 hours ago, so it is kept for history and starts no journey.',
    };
  return {
    tone: 'green',
    icon: 'check',
    title: 'Accepted',
    text: started
      ? `Started ${plural(started, 'journey execution')}.`
      : 'No published journey starts on this event for this customer.',
  };
});

onMounted(() => {
  load();
  loadSuggestions();
});
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>Events</h1>
      <p class="page-description">
        Every event your systems send lands here. Open one to see the journeys it moved.
      </p>
    </div>
    <button :disabled="loading" @click="load"><AppIcon name="refresh" :size="15" />Refresh</button>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <div class="events-layout" :class="{ 'is-readonly': readonly }">
    <section class="panel events-feed" aria-label="Recent events">
      <header class="events-feed-head">
        <h2>Recent events</h2>
        <span v-if="events.length" class="events-count">
          {{
            filtering
              ? `${filtered.length} of ${plural(events.length, 'event')}`
              : plural(events.length, 'event') + (more ? '+' : '')
          }}
        </span>
      </header>
      <div v-if="events.length || filtering" class="events-toolbar">
        <label class="events-search">
          <AppIcon name="search" :size="15" />
          <input v-model="search" aria-label="Find events" placeholder="Customer or event name" />
        </label>
        <div class="events-name-filter">
          <AppSelect
            v-model="nameFilter"
            caption="Event"
            aria-label="Filter by event name"
            :options="nameOptions"
          />
        </div>
        <SegmentedControl
          v-model="statusFilter"
          label="Status"
          hide-label
          :options="statusOptions"
        />
      </div>
      <SkeletonBlock v-if="loading && !events.length" label="Loading events…" :rows="6" />
      <EmptyState v-else-if="!events.length" icon="events" title="No events received yet" compact>
        Send one with the form, or post to <code>/api/v1/events</code>, and it appears here.
      </EmptyState>
      <div v-else-if="!filtered.length" class="events-none">
        <strong>No events match these filters</strong>
        <button class="link-button" @click="clearFilters">Clear filters</button>
      </div>
      <template v-else>
        <div class="events-columns" aria-hidden="true">
          <span>Event</span><span>Customer</span><span>Received</span><span>Status</span>
        </div>
        <section v-for="group in groups" :key="group.day" class="events-day">
          <h3>{{ group.day }}</h3>
          <button
            v-for="event in group.rows"
            :key="event.event_id"
            class="event-row"
            :class="{ 'is-new': fresh === event.event_id }"
            @click="inspect(event.event_id)"
          >
            <span class="event-name">
              <span
                class="event-mark"
                :class="{ provider: isDeliveryReport(event.event_name) }"
                :title="
                  isDeliveryReport(event.event_name)
                    ? 'Delivery report from the message provider'
                    : 'Event sent by your systems'
                "
                ><AppIcon
                  :name="isDeliveryReport(event.event_name) ? 'message' : 'events'"
                  :size="13"
              /></span>
              <strong>{{ event.event_name }}</strong>
            </span>
            <span class="event-customer">{{ event.customer_id }}</span>
            <span
              class="event-time"
              :title="`Occurred ${formatDateTime(event.occurred_at, { seconds: true })}`"
              >{{ clock(event.received_at) }}</span
            >
            <span class="event-status">
              <StatusBadge v-if="event.status === 'LATE'" status="WAITING" label="Late" />
              <span v-else class="event-accepted"
                ><span class="event-dot" aria-hidden="true" />Accepted</span
              >
              <span v-if="Number(event.duplicate_count) > 0" class="event-dupes">
                +{{ event.duplicate_count }}
                {{ event.duplicate_count === 1 ? 'repeat' : 'repeats' }}
              </span>
            </span>
          </button>
        </section>
        <button
          v-if="more && !filtering"
          class="events-more"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : 'Load older events' }}
        </button>
      </template>
    </section>
    <section v-if="!readonly" class="panel events-send" aria-labelledby="send-event-title">
      <h2 id="send-event-title">Send an event</h2>
      <p class="events-send-note">
        <AppIcon name="zap" :size="14" />This goes through the live pipeline and can start published
        journeys. For isolated runs, use Test Lab.
      </p>
      <form class="editor-form" novalidate @submit.prevent="send">
        <label
          >Customer ID<input
            v-model="customer"
            list="event-customers"
            required
            maxlength="128"
            autocomplete="off"
            :placeholder="customerIDs[0] ? `e.g. ${customerIDs[0]}` : 'Customer ID'"
        /></label>
        <datalist id="event-customers">
          <option v-for="id in customerIDs" :key="id" :value="id" />
        </datalist>
        <label
          >Event name<input
            v-model="name"
            list="event-names"
            required
            maxlength="128"
            autocomplete="off"
        /></label>
        <datalist id="event-names">
          <option v-for="n in eventNames" :key="n" :value="n" />
        </datalist>
        <div class="field">
          <label for="event-occurred">Occurred at</label>
          <div class="events-occurred">
            <input id="event-occurred" v-model="occurred" type="datetime-local" step="1" required />
            <button type="button" @click="occurred = localNow()">Now</button>
          </div>
          <small v-if="timing.text" :class="['events-hint', timing.tone]">{{ timing.text }}</small>
        </div>
        <div class="field">
          <div class="events-label-row">
            <label for="event-properties">Properties (JSON)</label>
            <button
              type="button"
              class="link-button"
              :disabled="!!parsed.error"
              @click="formatProperties"
            >
              Format
            </button>
          </div>
          <textarea
            id="event-properties"
            v-model="properties"
            class="code-input"
            rows="7"
            spellcheck="false"
            :aria-invalid="!!parsed.error"
            aria-describedby="event-properties-hint"
          />
          <small id="event-properties-hint" :class="['events-hint', parsed.error ? 'error' : '']">{{
            parsed.error ||
            'Journey conditions and messages read these, e.g. event.properties.amount.'
          }}</small>
        </div>
        <details class="events-advanced">
          <summary>
            Event ID <code>{{ eventID.slice(0, 8) }}…</code>
          </summary>
          <div class="event-id-field">
            <label>Event ID<input v-model="eventID" required maxlength="128" /></label
            ><button type="button" @click="newEvent">New event ID</button>
          </div>
          <small class="events-hint"
            >Sending the same ID again is ignored, so a retry never runs a step twice.</small
          >
        </details>
        <button class="primary events-submit" :disabled="!canSend">
          {{ sending ? 'Sending…' : 'Send event' }}
        </button>
      </form>
      <div role="status" class="events-result-slot">
        <div v-if="outcome && result" :class="['events-result', outcome.tone]">
          <span class="events-result-icon" aria-hidden="true"
            ><AppIcon :name="outcome.icon" :size="15"
          /></span>
          <div>
            <strong>{{ outcome.title }}</strong>
            <p>{{ outcome.text }}</p>
            <div class="events-result-actions">
              <button type="button" class="link-button" @click="inspect(result.event_id)">
                View event
              </button>
              <button type="button" class="link-button" @click="newEvent">Send another</button>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
<style scoped>
.events-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(320px, 1fr);
  gap: 20px;
  align-items: start;
}
.events-layout.is-readonly {
  grid-template-columns: minmax(0, 1fr);
}
.events-feed-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.events-feed-head h2,
.events-send h2 {
  font-size: 15px;
  margin: 0;
}
.events-count {
  font-size: 12px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
.events-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.events-search {
  display: flex;
  flex: 1 1 200px;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  background: var(--surface);
}
.events-search:focus-within {
  border-color: var(--accent);
}
.events-search input {
  flex: 1;
  min-width: 0;
  height: 34px;
  padding: 0;
  border: 0;
  outline: 0;
  background: none;
  color: var(--text);
  font-size: 13px;
}
.events-name-filter {
  flex: 0 1 210px;
  min-width: 160px;
}
.events-columns,
.event-row {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) 76px 148px;
  gap: 12px;
  align-items: center;
}
.events-columns {
  padding: 0 12px 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}
.events-day h3 {
  margin: 14px 12px 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}
.event-row {
  width: 100%;
  min-height: 44px;
  padding: 8px 12px;
  margin: 0;
  text-align: left;
  font-size: 13px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
}
.event-row:hover {
  background: var(--accent-soft);
}
.event-row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
.event-row.is-new {
  animation: event-new 2.4s ease-out;
}
@keyframes event-new {
  0%,
  40% {
    background: var(--accent-soft);
    box-shadow: inset 3px 0 0 var(--accent);
  }
}
@media (prefers-reduced-motion: reduce) {
  .event-row.is-new {
    animation: none;
    box-shadow: inset 3px 0 0 var(--accent);
  }
}
.event-name {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.event-name strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12.5px;
  font-weight: 600;
}
.event-mark {
  display: grid;
  flex: none;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 7px;
  color: var(--accent-text);
  background: var(--accent-soft);
}
.event-mark.provider {
  color: var(--muted);
  background: var(--gray-96);
}
.event-accepted {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}
.event-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--green-54);
}
.event-customer {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--muted);
}
.event-time {
  font-variant-numeric: tabular-nums;
  color: var(--muted);
}
.event-status {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.event-dupes {
  font-size: 11px;
  color: var(--muted);
}
.events-none {
  display: grid;
  justify-items: center;
  gap: 8px;
  padding: 32px 8px;
  font-size: 13px;
}
.events-more {
  display: block;
  margin: 14px auto 0;
}
.events-send {
  position: sticky;
  top: 16px;
}
.events-send-note {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  margin: 8px 0 18px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--muted);
}
.events-send-note :deep(svg) {
  flex: none;
  margin-top: 2px;
  color: var(--accent-text);
}
.field {
  display: grid;
  gap: 6px;
}
.events-occurred {
  display: flex;
  gap: 8px;
}
.events-occurred input {
  flex: 1;
  min-width: 0;
}
.events-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.events-hint {
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}
.events-hint.warn {
  color: var(--amber-55-text);
}
.events-hint.error {
  color: var(--red-56);
}
textarea[aria-invalid='true'] {
  border-color: var(--red-56);
}
.events-advanced {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 12px;
}
.events-advanced summary {
  cursor: pointer;
  font-size: 13px;
  color: var(--muted);
}
.events-advanced[open] summary {
  margin-bottom: 10px;
}
.events-advanced .event-id-field {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  margin-bottom: 6px;
}
.events-advanced .event-id-field label {
  flex: 1;
  min-width: 0;
}
.events-submit {
  width: 100%;
}
.events-result-slot:empty {
  display: none;
}
.events-result {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
}
.events-result.green {
  border-color: var(--green-74);
  background: var(--green-97);
}
.events-result.amber {
  border-color: var(--amber-77);
  background: var(--amber-97);
}
.events-result-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--on-solid);
  background: var(--green-54);
}
.events-result.amber .events-result-icon {
  background: var(--amber-62);
}
.events-result strong {
  font-size: 13px;
}
.events-result p {
  margin: 2px 0 6px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}
.events-result-actions {
  display: flex;
  gap: 14px;
}
@media (max-width: 1050px) {
  .events-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .events-send {
    position: static;
  }
}
@media (max-width: 760px) {
  .events-name-filter {
    flex-basis: 100%;
  }
  .events-columns {
    display: none;
  }
  .event-row {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas: 'name status' 'customer time';
    row-gap: 4px;
  }
  .event-name {
    grid-area: name;
  }
  .event-status {
    grid-area: status;
    justify-self: end;
  }
  .event-customer {
    grid-area: customer;
    padding-left: 33px;
    font-size: 12px;
  }
  .event-time {
    grid-area: time;
    justify-self: end;
    font-size: 12px;
  }
}
</style>
