<script setup lang="ts">
import EmptyState from './ui/EmptyState.vue';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { errorMessage, formatDateTime } from '../lib/format';
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue';
import type { InspectionKind } from '../lib/navigation';
import StatusBadge from './ui/StatusBadge.vue';
import { api } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import CustomerJourney, { type CustomerExecution } from './CustomerJourney.vue';
import { activityFeed, displayName, initials } from '../lib/customer-journey';
import { plural } from '../lib/format';
import { toast } from '../lib/toast';
type Customer = {
  customer_id: string;
  external_id?: string;
  email?: string;
  phone?: string;
  locale: string;
  timezone: string;
  // The list endpoint omits attributes and consents; the profile endpoint returns them.
  attributes: Record<string, unknown> | null;
  consents: Record<string, boolean> | null;
  created_at: string;
  updated_at: string;
};
const props = defineProps<{ readonly?: boolean; customerId?: string }>();
const emit = defineEmits<{
  select: [string | undefined];
  inspect: [{ kind: InspectionKind; id: string }];
  monitor: [{ journeyID: string; executionID: string }];
}>();
type EventRow = { event_id: string; event_name: string; occurred_at: string; status?: string };
type DeliveryRow = {
  id: string;
  channel: string;
  mode?: string;
  status: string;
  created_at?: string;
  accepted_at?: string | null;
};
const tabs = ['Journey', 'Activity', 'Profile'] as const;
const tab = ref<(typeof tabs)[number]>('Journey');
const creating = ref(false);
// A profile reads as a record first; editing is a deliberate step.
const editing = ref(false);
const journeyNames = ref<Record<string, string>>({});
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let request = 0;
let listRequest = 0;
let disposed = false;
const executions = ref<CustomerExecution[]>([]);
const customers = ref<Customer[]>([]);
const selected = ref<Customer>();
const search = ref('');
const error = ref('');
const busy = ref(false);
const loadingProfile = ref(false);
const attributes = ref('{}');
const events = ref<EventRow[]>([]);
const deliveries = ref<DeliveryRow[]>([]);
const form = ref({
  customer_id: '',
  external_id: '',
  email: '',
  phone: '',
  locale: 'en',
  timezone: 'UTC',
  emailConsent: false,
  smsConsent: false,
});
async function load() {
  const current = ++listRequest;
  busy.value = true;
  try {
    const result = await api<Customer[]>(
      `/customers?limit=200&search=${encodeURIComponent(search.value)}`,
    );
    if (!disposed && current === listRequest) customers.value = result;
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    if (!disposed && current === listRequest) busy.value = false;
  }
}
async function open(customer: Pick<Customer, 'customer_id'>) {
  const current = ++request;
  loadingProfile.value = true;
  creating.value = false;
  editing.value = false;
  if (selected.value?.customer_id !== customer.customer_id) tab.value = 'Journey';
  selected.value = undefined;
  error.value = '';
  events.value = [];
  deliveries.value = [];
  executions.value = [];
  try {
    const c = await api<Customer>(`/customers/${encodeURIComponent(customer.customer_id)}`);
    if (disposed || current !== request) return;
    selected.value = c;
    form.value = {
      customer_id: c.customer_id,
      external_id: c.external_id || '',
      email: c.email || '',
      phone: c.phone || '',
      locale: c.locale,
      timezone: c.timezone,
      emailConsent: !!c.consents?.email,
      smsConsent: !!c.consents?.sms,
    };
    attributes.value = JSON.stringify(c.attributes || {}, null, 2);
    const [timeline, history, runs] = await Promise.all([
      api<EventRow[]>(`/customers/${encodeURIComponent(c.customer_id)}/events`),
      api<DeliveryRow[]>(`/deliveries?customer_id=${encodeURIComponent(c.customer_id)}`),
      api<typeof executions.value>(
        `/customers/${encodeURIComponent(c.customer_id)}/executions?limit=25`,
      ),
    ]);
    if (disposed || current !== request) return;
    executions.value = runs;
    events.value = timeline;
    deliveries.value = history;
  } catch (cause) {
    if (!disposed && current === request) error.value = errorMessage(cause);
  } finally {
    if (!disposed && current === request) loadingProfile.value = false;
  }
}
function reset(fromRoute = false) {
  request++;
  editing.value = false;
  creating.value = !fromRoute;
  loadingProfile.value = false;
  error.value = '';
  deliveries.value = [];
  executions.value = [];
  if (!fromRoute) emit('select', undefined);
  selected.value = undefined;
  form.value = {
    customer_id: '',
    external_id: '',
    email: '',
    phone: '',
    locale: 'en',
    timezone: 'UTC',
    emailConsent: false,
    smsConsent: false,
  };
  attributes.value = '{}';
  events.value = [];
}
async function save() {
  if (props.readonly) return;
  error.value = '';
  const existing = !!selected.value;
  try {
    const c = await api<Customer>(
      selected.value
        ? `/customers/${encodeURIComponent(selected.value.customer_id)}`
        : '/customers',
      selected.value ? 'PUT' : 'POST',
      {
        customer_id: form.value.customer_id || undefined,
        external_id: form.value.external_id || undefined,
        email: form.value.email || undefined,
        phone: form.value.phone || undefined,
        locale: form.value.locale,
        timezone: form.value.timezone,
        attributes: JSON.parse(attributes.value),
        consents: { email: form.value.emailConsent, sms: form.value.smsConsent },
      },
    );
    toast(existing ? 'Profile saved' : 'Customer created');
    await load();
    if (props.customerId === c.customer_id) await open(c);
    else emit('select', c.customer_id);
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
const attributeRows = computed(() =>
  Object.entries(selected.value?.attributes || {}).map(([key, value]) => ({
    key,
    value: typeof value === 'string' ? value : JSON.stringify(value),
  })),
);
function cancelEdit() {
  editing.value = false;
  error.value = '';
  if (selected.value) void open(selected.value);
}
const name = computed(() => (selected.value ? displayName(selected.value) : ''));
const feed = computed(() =>
  activityFeed(
    events.value,
    deliveries.value,
    executions.value,
    (execution) =>
      journeyNames.value[
        executions.value.find((item) => item.id === execution.id)?.journey_id || ''
      ] || 'a journey',
  ),
);
const day = (value: string) =>
  new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
// Consecutive feed items that share a calendar day, newest day first.
const feedDays = computed(() => {
  const groups: { day: string; items: typeof feed.value }[] = [];
  for (const item of feed.value) {
    const label = day(item.at);
    if (groups.at(-1)?.day !== label) groups.push({ day: label, items: [] });
    groups.at(-1)!.items.push(item);
  }
  return groups;
});
const feedIcon = { event: 'events', message: 'email', execution: 'workflow' } as const;
const counts = computed(() => ({
  Journey: executions.value.length,
  Activity: feed.value.length,
  Profile: undefined,
}));
watch(search, () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => void load(), 250);
});
watch(
  () => props.customerId,
  (id) => {
    if (id) void open({ customer_id: id });
    else reset(true);
  },
);
onBeforeUnmount(() => {
  disposed = true;
  clearTimeout(searchTimer);
  request++;
  listRequest++;
});
onMounted(() => {
  void load();
  void api<{ id: string; name: string }[]>('/journeys?limit=200')
    .then((list) => {
      if (!disposed) journeyNames.value = Object.fromEntries(list.map((j) => [j.id, j.name]));
    })
    .catch(() => undefined);
  if (props.customerId) void open({ customer_id: props.customerId });
});
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">YOUR AUDIENCE</p>
      <h1>Customers</h1>
      <p class="page-description">
        Pick a person to see the route they took, what they received and what they did.
      </p>
    </div>
    <button v-if="!readonly" class="primary" @click="reset()">
      <AppIcon name="plus" :size="15" />New customer
    </button>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <div class="customers-layout" :class="{ 'has-detail': customerId || creating }">
    <aside class="panel people" aria-label="Customer list">
      <form class="people-search" role="search" @submit.prevent="load">
        <AppIcon name="search" :size="15" />
        <input
          v-model="search"
          aria-label="Find customer"
          placeholder="ID, external ID or email prefix"
        />
      </form>
      <SkeletonBlock v-if="busy && !customers.length" label="Loading customers…" :rows="6" />
      <div v-else-if="!customers.length" class="people-empty">
        <strong>{{ search ? 'No one matches that search' : 'No customers yet' }}</strong>
        <p>
          {{
            search
              ? 'Search matches the start of an ID, external ID or email.'
              : 'Customers appear when events arrive, or you can add one.'
          }}
        </p>
      </div>
      <template v-else>
        <p class="people-count">
          {{
            customers.length === 200
              ? 'First 200 profiles · search to narrow'
              : plural(customers.length, 'profile')
          }}
        </p>
        <button
          v-for="customer in customers"
          :key="customer.customer_id"
          class="person"
          :aria-pressed="customerId === customer.customer_id"
          @click="emit('select', customer.customer_id)"
        >
          <span class="avatar" aria-hidden="true">{{ initials(displayName(customer)) }}</span>
          <span class="person-copy"
            ><strong>{{ displayName(customer) }}</strong
            ><small
              >{{ customer.customer_id
              }}<template v-if="customer.external_id">
                · {{ customer.external_id }}</template
              ></small
            ></span
          >
        </button>
      </template>
    </aside>
    <section class="customer-detail">
      <button
        v-if="customerId || creating"
        class="back-to-list"
        type="button"
        @click="
          creating = false;
          emit('select', undefined);
        "
      >
        <AppIcon name="back" :size="15" />All customers
      </button>
      <div v-if="!customerId && !creating" class="panel customer-placeholder">
        <EmptyState icon="users" :level="2" title="Select a customer">
          Their journey map, messages and events open here.
        </EmptyState>
      </div>
      <div v-if="loadingProfile" class="panel">
        <SkeletonBlock label="Loading customer profile…" variant="text" :rows="6" />
      </div>
      <div v-if="customerId && !selected && !loadingProfile" class="panel">
        <EmptyState icon="refresh" :level="2" title="This profile did not load">
          <template #action>
            <button @click="open({ customer_id: customerId })">Retry customer</button>
          </template>
        </EmptyState>
      </div>
      <template v-if="selected && !loadingProfile">
        <header class="panel profile-head">
          <span class="avatar large" aria-hidden="true">{{ initials(name) }}</span>
          <div class="profile-identity">
            <h2>{{ name }}</h2>
            <p>
              {{ selected.customer_id
              }}<template v-if="selected.external_id"> · {{ selected.external_id }}</template
              ><template v-if="selected.email && selected.email !== name">
                · {{ selected.email }}</template
              >
            </p>
            <div class="profile-chips">
              <span :class="selected.consents?.email ? 'chip-yes' : 'chip-no'"
                ><AppIcon :name="selected.consents?.email ? 'check' : 'close'" :size="12" />Email
                {{ selected.consents?.email ? 'allowed' : 'not allowed' }}</span
              ><span :class="selected.consents?.sms ? 'chip-yes' : 'chip-no'"
                ><AppIcon :name="selected.consents?.sms ? 'check' : 'close'" :size="12" />SMS
                {{ selected.consents?.sms ? 'allowed' : 'not allowed' }}</span
              ><span><AppIcon name="clock" :size="12" />{{ selected.timezone }}</span
              ><span>{{ selected.locale }}</span>
            </div>
          </div>
          <dl class="profile-stats">
            <div>
              <dt>Journeys</dt>
              <dd>{{ executions.length }}</dd>
            </div>
            <div>
              <dt>Messages</dt>
              <dd>{{ deliveries.length }}</dd>
            </div>
            <div>
              <dt>Events</dt>
              <dd>{{ events.length }}</dd>
            </div>
          </dl>
        </header>
        <div class="detail-tabs" role="tablist" aria-label="Customer sections">
          <button
            v-for="item in tabs"
            :id="'customer-tab-' + item"
            :key="item"
            role="tab"
            type="button"
            :aria-selected="tab === item"
            aria-controls="customer-tabpanel"
            @click="tab = item"
          >
            {{ item }}<span v-if="counts[item] !== undefined">{{ counts[item] }}</span>
          </button>
        </div>
      </template>
      <div
        v-if="creating || (selected && !loadingProfile)"
        id="customer-tabpanel"
        class="panel"
        :role="creating ? undefined : 'tabpanel'"
        :aria-labelledby="creating ? undefined : 'customer-tab-' + tab"
      >
        <template v-if="!creating && tab === 'Journey'">
          <CustomerJourney
            v-if="executions.length"
            :executions="executions"
            :journey-names="journeyNames"
            @inspect="emit('inspect', $event)"
            @monitor="emit('monitor', $event)"
          />
          <EmptyState v-else icon="workflow" title="No journey entries yet">
            When an event enrolls this person, the route they take is drawn here.
          </EmptyState>
        </template>
        <template v-else-if="!creating && tab === 'Activity'">
          <EmptyState v-if="!feed.length" icon="events" title="Nothing has happened yet">
            Events, journey entries and messages for this person are listed here.
          </EmptyState>
          <div v-for="group in feedDays" :key="group.day" class="feed-day">
            <h3>{{ group.day }}</h3>
            <button
              v-for="item in group.items"
              :key="item.key"
              type="button"
              class="feed-item"
              @click="emit('inspect', { kind: item.kind, id: item.id })"
            >
              <span class="feed-mark" :class="item.kind"
                ><AppIcon
                  :name="
                    item.kind === 'message' && item.title.startsWith('SMS')
                      ? 'message'
                      : feedIcon[item.kind]
                  "
                  :size="15"
              /></span>
              <span class="feed-copy"
                ><strong>{{ item.title }}</strong
                ><small
                  >{{ item.detail }} · {{ formatDateTime(item.at, { seconds: true }) }}</small
                ></span
              >
              <StatusBadge v-if="item.status" :status="item.status" />
            </button>
          </div>
          <small v-if="feed.length" class="feed-note"
            >Latest 25 journey entries, with this person's events and messages.</small
          >
        </template>
        <template v-else-if="!creating && !editing && selected">
          <div class="record-heading">
            <div>
              <h2 class="form-title">Profile details</h2>
              <p>
                Your systems keep this record current through events and the API. Edit it here only
                to correct a value by hand.
              </p>
            </div>
            <button v-if="!readonly" type="button" @click="editing = true">Edit profile</button>
          </div>
          <dl class="record">
            <div>
              <dt>Customer ID</dt>
              <dd>{{ selected.customer_id }}</dd>
            </div>
            <div>
              <dt>External ID</dt>
              <dd>{{ selected.external_id || '—' }}</dd>
            </div>
            <div>
              <dt>Email</dt>
              <dd>{{ selected.email || '—' }}</dd>
            </div>
            <div>
              <dt>Phone</dt>
              <dd>{{ selected.phone || '—' }}</dd>
            </div>
            <div>
              <dt>Locale</dt>
              <dd>{{ selected.locale }}</dd>
            </div>
            <div>
              <dt>Timezone</dt>
              <dd>{{ selected.timezone }}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{{ formatDateTime(selected.created_at) }}</dd>
            </div>
            <div>
              <dt>Last updated</dt>
              <dd>{{ formatDateTime(selected.updated_at) }}</dd>
            </div>
          </dl>
          <h3 class="record-title">Custom attributes</h3>
          <p v-if="!attributeRows.length" class="record-none">
            No custom attributes on this profile.
          </p>
          <dl v-else class="record attributes">
            <div v-for="row in attributeRows" :key="row.key">
              <dt>{{ row.key }}</dt>
              <dd>{{ row.value }}</dd>
            </div>
          </dl>
        </template>
        <template v-else>
          <h2 class="form-title">{{ creating ? 'New customer' : 'Edit profile' }}</h2>
          <form class="editor-form profile-form" @submit.prevent="save">
            <label
              >Customer ID<input
                v-model="form.customer_id"
                :disabled="!!selected || readonly"
                placeholder="Generated if blank"
                maxlength="128" /></label
            ><label
              >External ID<input
                v-model="form.external_id"
                :disabled="readonly"
                maxlength="128" /></label
            ><label>Email<input v-model="form.email" type="email" :disabled="readonly" /></label
            ><label
              >Phone<input v-model="form.phone" :disabled="readonly" placeholder="+905551234567"
            /></label>
            <label>Locale<input v-model="form.locale" :disabled="readonly" /></label
            ><label>Timezone<input v-model="form.timezone" :disabled="readonly" /></label>
            <div class="consent-row wide">
              <label
                ><input v-model="form.emailConsent" type="checkbox" :disabled="readonly" /> Email
                consent</label
              ><label
                ><input v-model="form.smsConsent" type="checkbox" :disabled="readonly" /> SMS
                consent</label
              >
            </div>
            <label class="wide"
              >Custom attributes (JSON)<textarea
                v-model="attributes"
                rows="7"
                class="code-input"
                :disabled="readonly"
              />
            </label>
            <div v-if="!readonly" class="form-actions wide">
              <button class="primary">
                {{ selected ? 'Save profile' : 'Create customer' }}</button
              ><button v-if="selected" type="button" @click="cancelEdit">Cancel</button>
            </div>
          </form>
        </template>
      </div>
    </section>
  </div>
</template>
<style scoped>
.customers-layout {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}
.people {
  position: sticky;
  top: 16px;
  display: grid;
  gap: 4px;
  max-height: calc(100vh - 32px);
  overflow: auto;
  padding: 14px;
}
.people-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  margin-bottom: 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  background: var(--surface);
}
.people-search:focus-within {
  border-color: var(--accent);
}
.people-search input {
  flex: 1;
  min-width: 0;
  height: 36px;
  padding: 0;
  border: 0;
  outline: 0;
  background: none;
  color: var(--text);
  font-size: 13px;
}
.people-count {
  margin: 0 2px 6px;
  font-size: 12px;
  color: var(--muted);
}
.people-empty {
  padding: 24px 8px;
  text-align: center;
}
.people-empty strong {
  font-size: 13px;
}
.people-empty p {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}
.person {
  display: flex;
  justify-content: flex-start;
  gap: 10px;
  width: 100%;
  padding: 8px;
  border-color: transparent;
  border-radius: 8px;
  background: none;
  text-align: left;
}
.person:hover {
  background: var(--page);
  border-color: transparent;
}
.person[aria-pressed='true'] {
  background: var(--accent-soft);
}
.person-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.person-copy strong,
.person-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.person-copy strong {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}
.person-copy small {
  font-size: 12px;
  color: var(--muted);
}
.avatar {
  display: grid;
  place-items: center;
  flex: 0 0 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-text);
  font-size: 12px;
  font-weight: 600;
}
.avatar.large {
  flex-basis: 56px;
  height: 56px;
  font-size: 18px;
}
.customer-detail {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.back-to-list {
  display: none;
  justify-self: start;
}
.profile-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 16px;
}
.profile-identity {
  flex: 1 1 260px;
  min-width: 0;
}
.profile-identity h2 {
  margin: 0;
  font-size: 18px;
  overflow-wrap: anywhere;
}
.profile-identity p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.profile-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}
.profile-chips span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--page);
  color: var(--muted);
  font-size: 12px;
}
.profile-chips .chip-yes {
  background: var(--green-97);
  color: var(--green-47-text);
}
.profile-chips .chip-no {
  background: var(--amber-97);
  color: var(--amber-51-text);
}
.profile-stats {
  display: flex;
  gap: 8px;
  margin: 0;
}
.profile-stats div {
  min-width: 84px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
}
.profile-stats dt {
  font-size: 12px;
  color: var(--muted);
}
.profile-stats dd {
  margin: 2px 0 0;
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.detail-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--border);
}
.detail-tabs button {
  min-height: 38px;
  padding: 8px 12px;
  border: 0;
  border-bottom: 2px solid transparent;
  border-radius: 0;
  background: none;
  color: var(--muted);
  font-size: 13px;
}
.detail-tabs button:hover {
  color: var(--text);
  background: none;
}
.detail-tabs button[aria-selected='true'] {
  border-bottom-color: var(--accent);
  color: var(--accent-text);
  font-weight: 600;
}
.detail-tabs span {
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--page);
  font-size: 11px;
  font-weight: 500;
}
.form-title {
  margin: 0 0 16px;
  font-size: 15px;
}
.record-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.record-heading .form-title {
  margin-bottom: 4px;
}
.record-heading p,
.record-none {
  margin: 0;
  max-width: 60ch;
  font-size: 13px;
  line-height: 1.5;
  color: var(--muted);
}
.record {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 1px;
  margin: 0;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--border);
  overflow: hidden;
}
.record div {
  padding: 12px 14px;
  background: var(--surface);
}
.record dt {
  font-size: 12px;
  color: var(--muted);
}
.record dd {
  margin: 3px 0 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.record.attributes dt {
  font-family: ui-monospace, monospace;
}
.record-title {
  margin: 24px 0 10px;
  font-size: 13px;
}
.form-actions {
  display: flex;
  gap: 8px;
}
.profile-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 16px;
  max-width: 760px;
}
.profile-form .wide {
  grid-column: 1 / -1;
}
.profile-form .primary {
  justify-self: start;
}
.feed-day h3 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}
.feed-day + .feed-day {
  margin-top: 20px;
}
.feed-item {
  display: flex;
  justify-content: flex-start;
  gap: 12px;
  width: 100%;
  padding: 10px;
  border-color: transparent;
  border-radius: 8px;
  background: none;
  text-align: left;
}
.feed-item:hover {
  background: var(--page);
  border-color: transparent;
}
.feed-mark {
  display: grid;
  place-items: center;
  flex: 0 0 30px;
  height: 30px;
  border-radius: 8px;
  background: var(--green-97);
  color: var(--green-47-text);
}
.feed-mark.message {
  background: var(--blue-96);
  color: var(--blue-56-text);
}
.feed-mark.execution {
  background: var(--accent-soft);
  color: var(--accent-text);
}
.feed-copy {
  display: grid;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.feed-copy strong {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  overflow-wrap: anywhere;
}
.feed-copy small,
.feed-note {
  font-size: 12px;
  color: var(--muted);
}
.feed-note {
  display: block;
  margin-top: 16px;
}
@media (max-width: 900px) {
  .customers-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .people {
    position: static;
    max-height: none;
  }
  /* On a phone the list and the person take turns. */
  .has-detail .people {
    display: none;
  }
  .back-to-list {
    display: inline-flex;
  }
  .customers-layout:not(.has-detail) .customer-placeholder {
    display: none;
  }
  .profile-form {
    grid-template-columns: minmax(0, 1fr);
  }
  .profile-stats {
    width: 100%;
  }
  .profile-stats div {
    flex: 1;
    min-width: 0;
  }
  .person,
  .detail-tabs button {
    min-height: 44px;
  }
}
</style>
