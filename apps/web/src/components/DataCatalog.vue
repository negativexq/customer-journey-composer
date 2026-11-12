<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import type { DataEntry } from '../lib/data-catalog';
defineProps<{ readonly?: boolean }>();
const entries = ref<DataEntry[]>([]),
  search = ref(''),
  error = ref(''),
  notice = ref(''),
  busy = ref(false);
const blank = (): DataEntry => ({
  key: '',
  kind: 'event',
  event_name: '',
  path: '',
  label: '',
  description: '',
  data_type: 'event',
  aliases: [],
  enabled: true,
});
const draft = ref(blank()),
  aliases = ref('');
const visible = computed(() =>
  entries.value.filter((e) =>
    (e.label + ' ' + e.path + ' ' + e.event_name)
      .toLowerCase()
      .includes(search.value.toLowerCase()),
  ),
);
async function load() {
  try {
    entries.value = await api('/data-catalog');
  } catch (e) {
    error.value = String(e);
  }
}
function edit(entry: DataEntry) {
  draft.value = { ...entry, aliases: [...entry.aliases] };
  aliases.value = entry.aliases.join(', ');
}
async function save() {
  busy.value = true;
  error.value = '';
  try {
    const e = {
      ...draft.value,
      aliases: aliases.value
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean),
    };
    if (e.kind === 'event') {
      e.path = '';
      e.data_type = 'event';
    } else if (e.kind === 'customer_field') e.event_name = '';
    entries.value = await api('/data-catalog', 'PUT', { entries: [e] });
    draft.value = blank();
    aliases.value = '';
    notice.value = 'Data definition saved';
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}
async function discover() {
  busy.value = true;
  error.value = '';
  try {
    entries.value = await api('/data-catalog/discover', 'POST', {});
    notice.value =
      'Recent names and types imported. Review their business meaning; no customer values are sent to AI.';
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <section class="panel data-catalog" aria-label="Workspace data catalog">
    <div class="section-heading">
      <div>
        <h2>Workspace data</h2>
        <p>
          Give events and profile fields names your team understands. AI uses these definitions when
          interpreting a brief.
        </p>
      </div>
      <button :disabled="busy || readonly" @click="discover">Discover recent data</button>
    </div>
    <p class="help-text">
      Discovery checks up to 100 recent events and 100 customers. It imports names and types only,
      preserves edited definitions and does not run in your delivery pipeline.
    </p>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <label
      >Find a definition<input
        v-model="search"
        type="search"
        placeholder="Registration, cart amount, account activation…"
    /></label>
    <div class="table-scroll catalog-table">
      <table>
        <thead>
          <tr>
            <th>Business name</th>
            <th>Kind / type</th>
            <th>Availability</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in visible" :key="entry.key">
            <td>
              <strong>{{ entry.label }}</strong>
              <details>
                <summary>Technical definition</summary>
                <code>{{ entry.event_name }} {{ entry.path }}</code>
                <p>{{ entry.description }}</p>
                <small>{{ entry.aliases.join(' · ') }}</small>
              </details>
            </td>
            <td>
              {{ entry.kind.replaceAll('_', ' ') }} · {{ entry.data_type
              }}<small>{{ entry.source }}</small>
            </td>
            <td>{{ entry.enabled ? 'Available' : 'Disabled' }}</td>
            <td>
              <button v-if="entry.source !== 'system'" :disabled="readonly" @click="edit(entry)">
                Edit</button
              ><small v-else>Built in</small>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <form v-if="!readonly" class="catalog-definition editor-form" @submit.prevent="save">
      <h3>{{ draft.key ? 'Edit definition' : 'Add a definition' }}</h3>
      <p class="help-text">
        Define planned integration fields here too. A definition documents your data contract; it
        does not create an event or populate customer values.
      </p>
      <div class="catalog-form-grid">
        <label
          >Kind<select
            v-model="draft.kind"
            :disabled="!!draft.key"
            @change="draft.data_type = draft.kind === 'event' ? 'event' : 'string'"
          >
            <option value="event">Customer event</option>
            <option value="customer_field">Customer field</option>
            <option value="event_field">Event field</option>
          </select></label
        >
        <label
          >Business name<input
            v-model="draft.label"
            required
            maxlength="160"
            placeholder="Account activated"
        /></label>
        <label v-if="draft.kind !== 'customer_field'"
          >Event identifier<input
            v-model="draft.event_name"
            required
            :disabled="!!draft.key"
            maxlength="80"
            placeholder="account_activated"
        /></label>
        <label v-if="draft.kind !== 'event'"
          >Field path<input
            v-model="draft.path"
            required
            :disabled="!!draft.key"
            maxlength="200"
            :placeholder="
              draft.kind === 'customer_field'
                ? 'customer.attributes.activated'
                : 'event.properties.amount'
            "
        /></label>
        <label v-if="draft.kind !== 'event'"
          >Value type<select v-model="draft.data_type">
            <option value="string">Text</option>
            <option value="number">Number</option>
            <option value="boolean">Yes / no</option>
            <option value="mixed">Conflicting types — needs review</option>
          </select></label
        >
        <label
          >Also called<input
            v-model="aliases"
            placeholder="Activation, account enabled"
            maxlength="1000"
        /></label>
      </div>
      <label
        >Meaning<textarea
          v-model="draft.description"
          rows="2"
          maxlength="1000"
          placeholder="When does this happen, and what does the field mean?"
        />
      </label>
      <label class="checkbox-label"
        ><input v-model="draft.enabled" type="checkbox" />Available to AI</label
      >
      <div class="toolbar">
        <button class="primary" :disabled="busy">Save definition</button
        ><button
          type="button"
          @click="
            draft = blank();
            aliases = '';
          "
        >
          Clear
        </button>
      </div>
    </form>
  </section>
</template>
