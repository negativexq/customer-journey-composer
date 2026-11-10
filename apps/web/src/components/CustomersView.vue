<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue';
import type { InspectionKind } from '../lib/navigation';
import StatusBadge from './ui/StatusBadge.vue';
import { api } from '../lib/api';
type Customer = {
  customer_id: string;
  external_id?: string;
  email?: string;
  phone?: string;
  locale: string;
  timezone: string;
  attributes: Record<string, unknown>;
  consents: Record<string, boolean>;
  created_at: string;
  updated_at: string;
};
const props = defineProps<{ readonly?: boolean; customerId?: string }>();
const emit = defineEmits<{
  select: [string | undefined];
  inspect: [{ kind: InspectionKind; id: string }];
}>();
let request = 0;
let listRequest = 0;
let disposed = false;
const executions = ref<{ id: string; state: string; started_at: string }[]>([]);
const customers = ref<Customer[]>([]);
const selected = ref<Customer>();
const search = ref('');
const error = ref('');
const busy = ref(false);
const loadingProfile = ref(false);
const attributes = ref('{}');
const events = ref<Record<string, unknown>[]>([]);
const deliveries = ref<Record<string, unknown>[]>([]);
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
    error.value = String(cause);
  } finally {
    if (!disposed && current === listRequest) busy.value = false;
  }
}
async function open(customer: Pick<Customer, 'customer_id'>) {
  const current = ++request;
  loadingProfile.value = true;
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
      emailConsent: !!c.consents.email,
      smsConsent: !!c.consents.sms,
    };
    attributes.value = JSON.stringify(c.attributes, null, 2);
    const [timeline, history, runs] = await Promise.all([
      api<Record<string, unknown>[]>(`/customers/${encodeURIComponent(c.customer_id)}/events`),
      api<Record<string, unknown>[]>(
        `/deliveries?customer_id=${encodeURIComponent(c.customer_id)}`,
      ),
      api<typeof executions.value>(
        `/customers/${encodeURIComponent(c.customer_id)}/executions?limit=25`,
      ),
    ]);
    if (disposed || current !== request) return;
    executions.value = runs;
    events.value = timeline;
    deliveries.value = history;
  } catch (cause) {
    if (!disposed && current === request) error.value = String(cause);
  } finally {
    if (!disposed && current === request) loadingProfile.value = false;
  }
}
function reset(fromRoute = false) {
  request++;
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
    await load();
    if (props.customerId === c.customer_id) await open(c);
    else emit('select', c.customer_id);
  } catch (cause) {
    error.value = String(cause);
  }
}
function inspect(event: Record<string, unknown>) {
  emit('inspect', { kind: 'event', id: String(event.event_id) });
}
function inspectDelivery(id: unknown) {
  emit('inspect', { kind: 'message', id: String(id) });
}
watch(
  () => props.customerId,
  (id) => {
    if (id) void open({ customer_id: id });
    else reset(true);
  },
);
onBeforeUnmount(() => {
  disposed = true;
  request++;
  listRequest++;
});
onMounted(() => {
  void load();
  if (props.customerId) void open({ customer_id: props.customerId });
});
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">YOUR AUDIENCE</p>
      <h1>Customers</h1>
      <p class="page-description">Profiles, permissions, and the moments that matter.</p>
    </div>
    <button v-if="!readonly" @click="reset()">New customer</button>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <div class="resource-grid">
    <section class="panel">
      <form class="toolbar" @submit.prevent="load">
        <label
          >Find customer<input
            v-model="search"
            placeholder="Customer ID, external ID or email prefix" /></label
        ><button>Search</button>
      </form>
      <p v-if="busy" aria-busy="true">Loading customers…</p>
      <p v-else-if="!customers.length">No customers found.</p>
      <button
        v-for="customer in customers"
        :key="customer.customer_id"
        class="list-row"
        :aria-pressed="selected?.customer_id === customer.customer_id"
        @click="emit('select', customer.customer_id)"
      >
        <span>{{ customer.email || customer.customer_id }}</span
        ><small>{{ customer.external_id }}</small></button
      ><small>Showing up to 200 profiles. Use search to narrow results.</small>
    </section>
    <section class="panel">
      <h2>{{ selected ? 'Customer profile' : 'New customer' }}</h2>
      <p v-if="loadingProfile" role="status" aria-busy="true">Loading customer profile…</p>
      <button
        v-if="customerId && !selected && !loadingProfile"
        @click="open({ customer_id: customerId })"
      >
        Retry customer
      </button>
      <form
        v-if="!loadingProfile && (!customerId || selected)"
        class="editor-form"
        @submit.prevent="save"
      >
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
        <div class="toolbar">
          <label>Locale<input v-model="form.locale" :disabled="readonly" /></label
          ><label>Timezone<input v-model="form.timezone" :disabled="readonly" /></label>
        </div>
        <div class="consent-row">
          <label
            ><input v-model="form.emailConsent" type="checkbox" :disabled="readonly" /> Email
            consent</label
          ><label
            ><input v-model="form.smsConsent" type="checkbox" :disabled="readonly" /> SMS
            consent</label
          >
        </div>
        <label
          >Custom attributes (JSON)<textarea
            v-model="attributes"
            rows="7"
            class="code-input"
            :disabled="readonly"
          /></label
        ><button v-if="!readonly" class="primary">
          {{ selected ? 'Save profile' : 'Create customer' }}
        </button>
      </form>
      <template v-if="selected"
        ><h3>Journey executions</h3>
        <p v-if="!executions.length">No journey enrollments found.</p>
        <button
          v-for="execution in executions"
          :key="execution.id"
          class="list-row"
          @click="emit('inspect', { kind: 'execution', id: execution.id })"
        >
          <span
            >{{ execution.id.slice(0, 8) }} ·
            {{ new Date(execution.started_at).toLocaleString() }}</span
          ><StatusBadge :status="execution.state" /></button
        ><small>Latest 25 executions.</small>
        <h3>Message history</h3>
        <p v-if="!deliveries.length">No message actions yet.</p>
        <button
          v-for="item in deliveries"
          :key="String(item.id)"
          class="list-row"
          @click="inspectDelivery(item.id)"
        >
          <span>{{ item.channel }} · {{ item.mode }}</span
          ><span>{{ item.status }}</span>
        </button>

        <h3>Event timeline</h3>
        <p v-if="!events.length">No events received yet.</p>
        <button
          v-for="event in events"
          :key="String(event.event_id)"
          class="list-row"
          @click="inspect(event)"
        >
          <span>{{ event.event_name }}</span
          ><small>{{ new Date(String(event.occurred_at)).toLocaleString() }}</small>
        </button>
      </template>
    </section>
  </div>
</template>
