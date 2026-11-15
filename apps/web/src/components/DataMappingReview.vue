<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import { errorMessage } from '../lib/format';
import { computed, onMounted, ref, watch } from 'vue';
import { api } from '../lib/api';
import type { Workflow } from '../lib/workflow';
import type { DataEntry, DataMapping, DataResolution, GroundingReport } from '../lib/data-catalog';
const props = defineProps<{ workflow: Workflow; confirmed?: string[]; readonly?: boolean }>();
const emit = defineEmits<{ ready: [boolean]; resolved: [Workflow, string[]] }>();
const report = ref<GroundingReport>(),
  entries = ref<DataEntry[]>([]),
  error = ref(''),
  busy = ref(false),
  choices = ref<Record<string, string>>({});
let request = 0;
const unresolved = computed(
  () => report.value?.mappings.filter((m) => !['matched', 'confirmed'].includes(m.status)) || [],
);
const options = (m: DataMapping) =>
  entries.value.filter(
    (e) =>
      e.enabled &&
      e.kind === m.reference.kind &&
      (e.kind !== 'event_field' || e.event_name === m.reference.event_name) &&
      (!m.reference.path?.startsWith('customer.attributes.') ||
        e.path?.startsWith('customer.attributes.')),
  );
async function refresh() {
  const ticket = ++request;
  emit('ready', false);
  busy.value = true;
  error.value = '';
  try {
    const result = await api<DataResolution>('/data-catalog/resolve', 'POST', {
      workflow: props.workflow,
      bindings: [],
      confirmed: props.confirmed || [],
    });
    if (ticket !== request) return;
    report.value = result.grounding;
    choices.value = {};
    emit('ready', result.grounding.ready);
  } catch (e) {
    if (ticket === request) error.value = errorMessage(e);
  } finally {
    if (ticket === request) busy.value = false;
  }
}
async function resolve() {
  busy.value = true;
  error.value = '';
  emit('ready', false);
  try {
    const bindings = Object.entries(choices.value)
      .filter(([, v]) => !!v)
      .map(([reference, target]) => ({ reference, target }));
    const result = await api<DataResolution>('/data-catalog/resolve', 'POST', {
      workflow: props.workflow,
      bindings,
      confirmed: props.confirmed || [],
    });
    const confirmed = result.grounding.mappings
      .filter(
        (m) =>
          m.status === 'ambiguous' &&
          (choices.value[m.reference.key] || bindings.some((b) => b.target === m.reference.key)),
      )
      .map((m) => m.reference.key);
    emit('resolved', result.workflow, [...new Set([...(props.confirmed || []), ...confirmed])]);
  } catch (e) {
    error.value = errorMessage(e);
    emit('ready', false);
  } finally {
    busy.value = false;
  }
}
watch(() => JSON.stringify([props.workflow, props.confirmed]), refresh);
async function reloadCatalog() {
  try {
    entries.value = await api('/data-catalog');
    await refresh();
  } catch (e) {
    error.value = errorMessage(e);
    emit('ready', false);
  }
}
onMounted(async () => {
  try {
    entries.value = await api('/data-catalog');
  } catch (e) {
    error.value = errorMessage(e);
  }
  await refresh();
});
</script>
<template>
  <section class="data-mapping-review" aria-label="Data used by this plan">
    <div class="section-heading">
      <div>
        <h3>Data used by this plan</h3>
        <p>Check how your brief maps to workspace data.</p>
      </div>
      <button type="button" :disabled="busy" @click="reloadCatalog">Refresh data</button>
    </div>
    <p v-if="busy" role="status">Checking workspace definitions…</p>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="report && !busy" :class="report.ready ? 'help-text' : 'warning-message'">
      {{
        report.ready
          ? 'All references have an available definition. Review the business meaning before approving.'
          : 'Resolve missing, ambiguous or conflicting data before approving this plan.'
      }}
    </p>
    <ul v-if="report" class="data-mapping-list">
      <li v-for="mapping in report.mappings" :key="mapping.reference.key">
        <div>
          <strong>{{ mapping.label }}</strong
          ><span class="pill">{{
            mapping.status === 'matched'
              ? 'Available'
              : mapping.status === 'confirmed'
                ? 'Your choice'
                : mapping.status === 'missing'
                  ? 'Not connected'
                  : mapping.status === 'ambiguous'
                    ? 'Choose a definition'
                    : mapping.status === 'type_mismatch'
                      ? 'Incompatible value type'
                      : 'Conflicting types'
          }}</span>
        </div>
        <details>
          <summary>Technical mapping</summary>
          <code>{{ mapping.reference.event_name }} {{ mapping.reference.path }}</code>
        </details>
        <label v-if="!['matched', 'confirmed'].includes(mapping.status)"
          >Use workspace definition<AppSelect
            v-model="choices[mapping.reference.key]"
            :aria-label="`Definition for ${mapping.reference.key}`"
            :disabled="readonly || busy"
            placeholder="Choose a definition"
            :options="
              options(mapping).map((entry) => ({
                value: entry.key,
                label: entry.label,
                description: entry.event_name || entry.path,
                icon:
                  entry.kind === 'event'
                    ? 'events'
                    : entry.kind === 'customer_field'
                      ? 'user'
                      : 'file',
              }))
            "
        /></label>
      </li>
    </ul>
    <template v-if="unresolved.length"
      ><p class="help-text">
        Missing a definition? Add or discover it in Settings → Workspace data, then refresh here.
        Changing an event may require choosing its fields again.
      </p>
      <button
        type="button"
        :disabled="busy || readonly || !Object.values(choices).some(Boolean)"
        @click="resolve"
      >
        Use selected definitions
      </button></template
    >
  </section>
</template>
