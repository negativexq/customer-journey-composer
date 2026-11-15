<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import { api } from '../lib/api';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { toast } from '../lib/toast';

// The live journey's audience schedule: when it next runs, what recent runs found, and a way to
// enroll the audience now. Renders nothing for journeys that start on an event.
const props = defineProps<{ journeyId: string; canRun?: boolean }>();
const emit = defineEmits<{ ran: [] }>();
type Run = {
  id: string;
  slot_at: string;
  trigger: 'SCHEDULE' | 'MANUAL';
  state: 'RUNNING' | 'DONE' | 'FAILED';
  checked: number;
  matched: number;
  enrolled: number;
};
type Status = { enabled: boolean; version?: number; next_run_at?: string; runs: Run[] };
const status = ref<Status>();
const error = ref('');
const running = ref(false);
let sequence = 0;

async function load() {
  const current = ++sequence;
  try {
    const next = await api<Status>(`/journeys/${props.journeyId}/audience`);
    if (current === sequence) {
      status.value = next;
      error.value = '';
    }
  } catch (cause) {
    if (current === sequence) error.value = errorMessage(cause);
  }
}
async function runNow() {
  running.value = true;
  error.value = '';
  try {
    const run = await api<Run>(`/journeys/${props.journeyId}/audience/runs`, 'POST');
    toast(`${plural(run.enrolled, 'customer')} entered · ${run.matched} matched the audience`);
    await load();
    emit('ran');
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    running.value = false;
  }
}
watch(() => props.journeyId, load, { immediate: true });
onBeforeUnmount(() => sequence++);
defineExpose({ reload: load });
</script>
<template>
  <section
    v-if="status?.enabled"
    class="surface audience-entry"
    aria-labelledby="audience-entry-title"
  >
    <div class="surface-heading">
      <div>
        <h3 id="audience-entry-title">Audience entry</h3>
        <p>
          Version {{ status.version }} enrolls matching customers on a schedule.
          <template v-if="status.next_run_at"
            >Next run {{ formatDateTime(status.next_run_at) }}.</template
          >
        </p>
      </div>
      <button v-if="canRun" type="button" :disabled="running" @click="runNow">
        <AppIcon name="users" :size="15" />{{
          running ? 'Enrolling…' : 'Enter matching customers now'
        }}
      </button>
    </div>
    <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
    <ol v-if="status.runs.length" class="audience-runs">
      <li v-for="run in status.runs" :key="run.id" :class="run.state.toLowerCase()">
        <span class="run-when"
          >{{ formatDateTime(run.slot_at) }}
          <small>{{ run.trigger === 'MANUAL' ? 'Started by hand' : 'Scheduled' }}</small></span
        >
        <span class="run-counts"
          ><strong>{{ plural(run.enrolled, 'customer') }} entered</strong> ·
          {{ run.matched }} matched of {{ run.checked }} checked<template
            v-if="run.state !== 'DONE'"
          >
            · {{ run.state === 'FAILED' ? 'stopped early' : 'running' }}</template
          ></span
        >
      </li>
    </ol>
    <p v-else class="audience-empty">No runs yet.</p>
  </section>
</template>
<style scoped>
.audience-entry {
  display: grid;
  gap: 12px;
}
.audience-entry .surface-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
}
.audience-entry .surface-heading p {
  margin: 4px 0 0;
}
.audience-runs {
  display: grid;
  gap: 0;
  margin: 0;
  padding: 0 22px 8px;
  list-style: none;
}
.audience-runs li {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  justify-content: space-between;
  padding: 10px 0;
  border-top: 1px solid var(--border);
  font-size: 13px;
}
.audience-runs .run-when small {
  margin-left: 6px;
  color: var(--muted);
}
.audience-runs li.failed .run-counts {
  color: var(--red-67-text);
}
.audience-entry > .inline-alert {
  margin: 0 22px;
}
.audience-empty {
  margin: 0;
  padding: 0 22px 18px;
  color: var(--muted);
}
</style>
