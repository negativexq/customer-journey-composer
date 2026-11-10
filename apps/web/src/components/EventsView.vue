<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../lib/api';
defineProps<{ readonly?: boolean }>();
const events = ref<Record<string, unknown>[]>([]);
const payload = ref<Record<string, unknown>>();
const customer = ref('');
const name = ref('cart_updated');
const eventID = ref(crypto.randomUUID() as string);
const occurred = ref(new Date().toISOString());
const properties = ref('{"cart_id":"cart_demo","amount":2450,"currency":"TRY"}');
const error = ref('');
const notice = ref('');
async function load() {
  try {
    events.value = await api('/events?limit=100');
  } catch (cause) {
    error.value = String(cause);
  }
}
async function inspect(id: unknown) {
  try {
    payload.value = await api(`/events/${encodeURIComponent(String(id))}`);
  } catch (cause) {
    error.value = String(cause);
  }
}
async function send() {
  error.value = '';
  try {
    const result = await api<{ status: string; executions: string[] }>('/events', 'POST', {
      event_id: eventID.value,
      customer_id: customer.value,
      event_name: name.value,
      occurred_at: new Date(occurred.value).toISOString(),
      properties: JSON.parse(properties.value),
    });
    notice.value = `${result.status} · ${result.executions.length} executions started`;
    await load();
  } catch (cause) {
    error.value = String(cause);
  }
}
function newEvent() {
  eventID.value = crypto.randomUUID();
  occurred.value = new Date().toISOString();
}
onMounted(load);
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">CUSTOMER MOMENTS</p>
      <h1>Events</h1>
      <p class="page-description">
        Inspect incoming activity and follow each entry into a journey.
      </p>
    </div>
    <button @click="load">Refresh</button>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <p role="status">{{ notice }}</p>
  <div class="resource-grid">
    <section class="panel">
      <h2>Recent events</h2>
      <p v-if="!events.length">No events received yet.</p>
      <button
        v-for="event in events"
        :key="String(event.event_id)"
        class="list-row event-row"
        @click="inspect(event.event_id)"
      >
        <span
          ><strong>{{ event.event_name }}</strong
          ><small>{{ event.customer_id }}</small></span
        ><span
          >{{ event.status
          }}<small>{{
            Number(event.duplicate_count) > 0
              ? `${event.duplicate_count} duplicate(s)`
              : new Date(String(event.received_at)).toLocaleString()
          }}</small></span
        >
      </button>
      <details v-if="payload" open>
        <summary>Event details</summary>
        <pre>{{ JSON.stringify(payload, null, 2) }}</pre>
      </details>
    </section>
    <section v-if="!readonly" class="panel">
      <h2>Send an event</h2>
      <p>This enters the live event pipeline. For isolated scenarios, use Test Lab.</p>
      <form class="editor-form" @submit.prevent="send">
        <label>Event ID<input v-model="eventID" required maxlength="128" /></label
        ><button type="button" @click="newEvent">New event ID</button
        ><label>Customer ID<input v-model="customer" required maxlength="128" /></label
        ><label>Event name<input v-model="name" required /></label
        ><label>Occurred at<input v-model="occurred" required /></label
        ><label
          >Properties (JSON)<textarea v-model="properties" class="code-input" rows="8" /></label
        ><button class="primary">Send event</button>
      </form>
      <small
        >Events older than 24 hours are stored as LATE and do not drive execution. Future timestamps
        beyond 5 minutes are rejected. Duplicate IDs do not create another logical action.</small
      >
    </section>
  </div>
</template>
