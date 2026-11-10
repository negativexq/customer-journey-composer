<script setup lang="ts">
import { onMounted, ref } from 'vue';
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
const props = defineProps<{ readonly?: boolean }>();
const customers = ref<Customer[]>([]);
const selected = ref<Customer>();
const search = ref('');
const error = ref('');
const busy = ref(false);
const attributes = ref('{}');
const events = ref<Record<string, unknown>[]>([]);
const payload = ref<Record<string, unknown>>();
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
  busy.value = true;
  try {
    customers.value = await api(`/customers?limit=200&search=${encodeURIComponent(search.value)}`);
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
async function open(customer: Customer) {
  try {
    const c = await api<Customer>(`/customers/${encodeURIComponent(customer.customer_id)}`);
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
    events.value = await api(`/customers/${encodeURIComponent(c.customer_id)}/events`);
  } catch (cause) {
    error.value = String(cause);
  }
}
function reset() {
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
    await open(c);
  } catch (cause) {
    error.value = String(cause);
  }
}
async function inspect(event: Record<string, unknown>) {
  try {
    payload.value = await api(`/events/${encodeURIComponent(String(event.event_id))}`);
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(load);
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">YOUR AUDIENCE</p>
      <h1>Customers</h1>
      <p class="page-description">Profiles, permissions, and the moments that matter.</p>
    </div>
    <button v-if="!readonly" @click="reset">New customer</button>
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
        @click="open(customer)"
      >
        <span>{{ customer.email || customer.customer_id }}</span
        ><small>{{ customer.external_id }}</small></button
      ><small>Showing up to 200 profiles. Use search to narrow results.</small>
    </section>
    <section class="panel">
      <h2>{{ selected ? 'Customer profile' : 'New customer' }}</h2>
      <form class="editor-form" @submit.prevent="save">
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
        ><h3>Event timeline</h3>
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
        <details v-if="payload" open>
          <summary>Event payload</summary>
          <pre>{{ JSON.stringify(payload, null, 2) }}</pre>
        </details></template
      >
    </section>
  </div>
</template>
