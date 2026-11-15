<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import { errorMessage } from '../lib/format';
import { computed, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import EmptyState from './ui/EmptyState.vue';
import { toast } from '../lib/toast';
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
const kinds = [
  { id: '', label: 'All' },
  { id: 'event', label: 'Events' },
  { id: 'customer_field', label: 'Customer fields' },
  { id: 'event_field', label: 'Event fields' },
] as const;
const kindLabel: Record<string, string> = {
  event: 'Event',
  customer_field: 'Customer field',
  event_field: 'Event field',
};
const typeLabel: Record<string, string> = {
  string: 'Text',
  number: 'Number',
  boolean: 'Yes / no',
  mixed: 'Conflicting types',
  event: '',
};
const kind = ref<(typeof kinds)[number]['id']>('');
// The definition form opens only while someone is adding or editing.
const editing = ref(false);
const visible = computed(() =>
  entries.value.filter(
    (e) =>
      (!kind.value || e.kind === kind.value) &&
      (e.label + ' ' + e.path + ' ' + e.event_name)
        .toLowerCase()
        .includes(search.value.toLowerCase()),
  ),
);
function close() {
  editing.value = false;
  draft.value = blank();
  aliases.value = '';
}
function add() {
  close();
  editing.value = true;
}
async function load() {
  try {
    entries.value = await api('/data-catalog');
  } catch (e) {
    error.value = errorMessage(e);
  }
}
function edit(entry: DataEntry) {
  draft.value = { ...entry, aliases: [...entry.aliases] };
  aliases.value = entry.aliases.join(', ');
  editing.value = true;
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
    close();
    notice.value = '';
    toast('Data definition saved');
  } catch (e) {
    error.value = errorMessage(e);
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
    error.value = errorMessage(e);
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <section class="data-catalog" aria-label="Workspace data catalog">
    <header class="settings-heading with-action">
      <div>
        <h2>Workspace data</h2>
        <p>
          Give events and profile fields names your team understands. AI uses these definitions when
          interpreting a brief.
        </p>
      </div>
      <button :disabled="busy || readonly" @click="discover">
        <AppIcon name="refresh" :size="15" />Discover recent data
      </button>
    </header>
    <p class="settings-help">
      Discovery checks up to 100 recent events and 100 customers. It imports names and types only,
      preserves edited definitions and does not run in your delivery pipeline.
    </p>
    <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
    <p v-if="notice" role="status" class="settings-status on">
      <span class="small-dot" />{{ notice }}
    </p>
    <form
      v-if="editing && !readonly"
      class="settings-card catalog-definition"
      @submit.prevent="save"
    >
      <h3>{{ draft.key ? 'Edit definition' : 'Add a definition' }}</h3>
      <p class="settings-help">
        Define planned integration fields here too. A definition documents your data contract; it
        does not create an event or populate customer values.
      </p>
      <div class="settings-fields">
        <label
          >Kind<AppSelect
            v-model="draft.kind"
            :disabled="!!draft.key"
            :options="[
              { value: 'event', label: 'Customer event' },
              { value: 'customer_field', label: 'Customer field' },
              { value: 'event_field', label: 'Event field' },
            ]"
            @change="draft.data_type = draft.kind === 'event' ? 'event' : 'string'"
        /></label>
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
          >Value type<AppSelect
            v-model="draft.data_type"
            :options="[
              { value: 'string', label: 'Text' },
              { value: 'number', label: 'Number' },
              { value: 'boolean', label: 'Yes / no' },
              { value: 'mixed', label: 'Conflicting types — needs review' },
            ]"
        /></label>
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
      <div class="settings-actions">
        <button class="primary" :disabled="busy">Save definition</button
        ><button type="button" @click="close">Cancel</button>
      </div>
    </form>
    <div class="settings-card">
      <div class="catalog-toolbar">
        <label class="catalog-search"
          >Find a definition<input
            v-model="search"
            type="search"
            placeholder="Registration, cart amount, account activation…"
        /></label>
        <div class="catalog-kinds" role="group" aria-label="Definition kind">
          <button
            v-for="item in kinds"
            :key="item.id"
            type="button"
            :aria-pressed="kind === item.id"
            @click="kind = item.id"
          >
            {{ item.label }}
          </button>
        </div>
        <button v-if="!readonly && !editing" class="primary" type="button" @click="add">
          <AppIcon name="plus" :size="15" />Add definition
        </button>
      </div>
      <EmptyState v-if="!visible.length" icon="templates" title="No definitions match" compact>
        Try another name or kind, or discover recent data.
      </EmptyState>
      <div v-else class="table-scroll catalog-table">
        <table>
          <thead>
            <tr>
              <th>Business name</th>
              <th>Kind</th>
              <th>AI</th>
              <th><span class="sr-only">Actions</span></th>
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
                {{ kindLabel[entry.kind] || entry.kind
                }}<small v-if="typeLabel[entry.data_type]">{{ typeLabel[entry.data_type] }}</small>
              </td>
              <td>
                <span class="settings-tag" :class="entry.enabled ? 'on' : 'off'">{{
                  entry.enabled ? 'Available' : 'Disabled'
                }}</span>
              </td>
              <td class="catalog-action">
                <button v-if="entry.source !== 'system'" :disabled="readonly" @click="edit(entry)">
                  Edit</button
                ><small v-else>Built in</small>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
<style scoped>
.data-catalog {
  display: grid;
  gap: 16px;
}
.catalog-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 12px;
  margin-bottom: 16px;
}
.catalog-search {
  flex: 1 1 240px;
}
.catalog-kinds {
  display: flex;
  padding: 3px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--page);
}
.catalog-kinds button {
  min-height: 30px;
  padding: 4px 10px;
  border: 0;
  background: none;
  color: var(--muted);
  white-space: nowrap;
}
.catalog-kinds button[aria-pressed='true'] {
  background: var(--surface);
  color: var(--text);
  font-weight: 600;
  box-shadow: 0 1px 2px #17213b14;
}
.catalog-action {
  text-align: right;
}
.catalog-definition {
  display: grid;
  gap: 12px;
}
.catalog-definition h3,
.catalog-definition .settings-help {
  margin: 0;
}
@media (max-width: 700px) {
  .catalog-kinds {
    width: 100%;
    overflow-x: auto;
  }
  .catalog-kinds button {
    min-height: 40px;
  }
}
</style>
