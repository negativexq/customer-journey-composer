<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{ journeyId?: string; overview?: boolean }>();
type Metric = { dimension: string; key: string; metric: string; value: number };
type Summary = { metrics: Metric[]; updated_at?: string; semantics: string };
type Daily = { day: string; dimension: string; key: string; metric: string; value: number };
const summary = ref<Summary>();
const recentEntries = ref(0);
const activity = ref<{
  recent_executions: { id: string; journey_id: string; customer_id: string; state: string }[];
  failed_executions: { id: string; journey_id: string; customer_id: string; state: string }[];
}>();
const comparison = ref<
  { id: string; name: string; entries: number; converted: number; active: number }[]
>([]);
const nodeMetrics = ref<{ node_id: string; dimension: string; metric: string; value: number }[]>(
  [],
);
const funnelVersion = ref(0);
const daily = ref<{ daily: Daily[]; semantics: string; updated_at?: string }>();
const journeys = ref<Journey[]>([]);
const selected = ref(props.journeyId || '');
const days = ref(30);
const error = ref('');
const busy = ref(false);
const emit = defineEmits<{ openJourney: [string]; browseJourneys: [string]; navigate: [string] }>();
const value = (dimension: string, metric: string, key = '') =>
  summary.value?.metrics.find(
    (m) => m.dimension === dimension && m.metric === metric && m.key === key,
  )?.value || 0;
const entries = computed(() => value('execution', 'entries'));
const converted = computed(() => value('execution', 'converted'));
const rate = computed(() =>
  entries.value ? ((100 * converted.value) / entries.value).toFixed(1) : '0.0',
);
const dates = computed(() => [...new Set(daily.value?.daily.map((d) => d.day) || [])]);
function at(day: string, metric: string) {
  return (
    daily.value?.daily.find(
      (d) => d.day === day && d.dimension === 'execution' && d.metric === metric,
    )?.value || 0
  );
}
const maxEntry = computed(() => Math.max(1, ...dates.value.map((d) => at(d, 'entries'))));
const funnel = computed(() =>
  nodeMetrics.value.filter((m) => m.dimension === 'node' && m.metric === 'entered'),
);
const nodeValue = (id: string, metric: string) =>
  nodeMetrics.value.find((m) => m.node_id === id && m.metric === metric)?.value || 0;
const funnelEntries = computed(
  () =>
    nodeMetrics.value.find((m) => m.dimension === 'execution' && m.metric === 'entries')?.value ||
    0,
);
async function load() {
  busy.value = true;
  error.value = '';
  try {
    const base = selected.value ? `/journeys/${selected.value}` : '';
    [summary.value, daily.value] = await Promise.all([
      api<Summary>(base ? base + '/stats' : '/overview'),
      api<typeof daily.value>(
        base ? base + `/analytics?days=${days.value}` : `/analytics?days=${days.value}`,
      ),
    ]);
    if (selected.value) {
      const versions = await api<{ version: number }[]>(
        '/journeys/' + selected.value + '/versions',
      );
      funnelVersion.value = versions[0]?.version || 0;
      nodeMetrics.value = funnelVersion.value
        ? await api(`/journeys/${selected.value}/node-stats?version=${funnelVersion.value}`)
        : [];
    } else {
      [comparison.value, activity.value] = await Promise.all([
        api<typeof comparison.value>('/analytics/comparison'),
        api<typeof activity.value>('/overview/activity'),
      ]);
      recentEntries.value = (await api<{ entries: number }>('/overview/entries-24h')).entries;
    }
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
onMounted(() => {
  void load();
  if (!props.journeyId)
    void api<Journey[]>('/journeys?limit=200')
      .then((j) => (journeys.value = j))
      .catch((cause) => (error.value = String(cause)));
});
</script>
<template>
  <section :aria-busy="busy">
    <div class="page-heading">
      <div>
        <h1>{{ overview ? 'Workspace overview' : 'Analytics' }}</h1>
        <p class="page-description">
          Execution-backed metrics · eventual consistency · UTC cohorts
        </p>
      </div>
      <button :disabled="busy" @click="load">Refresh metrics</button>
    </div>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <div v-if="!journeyId" class="editor-form">
      <label
        >Analytics scope<select v-model="selected" @change="load">
          <option value="">Entire workspace</option>
          <option v-for="j in journeys" :key="j.id" :value="j.id">{{ j.name }}</option>
        </select></label
      ><label
        >UTC calendar days<select v-model.number="days" @change="load">
          <option :value="7">7</option>
          <option :value="30">30</option>
          <option :value="90">90</option>
        </select></label
      >
    </div>
    <p v-if="!summary?.updated_at">
      Metrics are waiting for the analytics worker. Refresh after processing.
    </p>
    <template v-if="summary"
      ><p>Processed {{ summary.updated_at || 'not yet' }}. {{ summary.semantics }}</p>
      <section class="stats-grid">
        <div v-if="!selected" class="panel stat-card">
          <span>Entries · last 24h</span><strong>{{ recentEntries }}</strong>
        </div>
        <button v-if="!selected" class="panel stat-card" @click="emit('browseJourneys', '')">
          <span>Total journeys</span><strong>{{ value('journey', 'total') }}</strong>
        </button>
        <button
          v-for="s in !selected
            ? ['Active', 'Draft', 'Paused', 'Sunset', 'Completed', 'Archived']
            : []"
          :key="s"
          class="panel stat-card"
          @click="emit('browseJourneys', s)"
        >
          <span>{{ s }} journeys</span><strong>{{ value('journey', s) }}</strong>
        </button>
        <div class="panel stat-card">
          <span>Logical entries · lifetime</span><strong>{{ entries }}</strong>
        </div>
        <div class="panel stat-card">
          <span>Unique enrolled customers · lifetime</span
          ><strong>{{ value('execution', 'unique_customers') }}</strong>
        </div>
        <div class="panel stat-card">
          <span>Active executions</span
          ><strong>{{ value('execution', 'RUNNING') + value('execution', 'WAITING') }}</strong>
        </div>
        <div class="panel stat-card">
          <span>Completed executions</span><strong>{{ value('execution', 'COMPLETED') }}</strong>
        </div>
        <div class="panel stat-card">
          <span>Converted / logical entries</span
          ><strong>{{ converted }} / {{ entries }} · {{ rate }}%</strong>
        </div>
        <div class="panel stat-card">
          <span>Average terminal duration (all outcomes)</span
          ><strong
            >{{
              (
                value('execution', 'duration_ms') /
                Math.max(1, value('execution', 'finished_count')) /
                1000
              ).toFixed(1)
            }}s</strong
          >
        </div>
      </section>
      <section class="panel">
        <h2>Entry &amp; conversion trend</h2>
        <p>{{ daily?.semantics }}</p>
        <p v-if="!dates.length">No entries in this period.</p>
        <div v-for="day in dates" :key="day" class="trend-row">
          <span>{{ day }}</span>
          <div class="trend-bar" :style="{ width: (at(day, 'entries') / maxEntry) * 60 + '%' }" />
          <span
            >{{ at(day, 'entries') }} entries · {{ at(day, 'converted') }} converted ·
            {{
              at(day, 'entries')
                ? ((100 * at(day, 'converted')) / at(day, 'entries')).toFixed(1)
                : 0
            }}%</span
          >
        </div>
      </section>
      <section class="panel">
        <h2>Channel performance · logical sends</h2>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Channel</th>
                <th>Scheduled</th>
                <th>Attempted</th>
                <th>Provider accepted</th>
                <th>Blocked</th>
                <th>Skipped / cancelled</th>
                <th>Failed</th>
                <th>Unknown</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in ['email', 'sms']" :key="c">
                <td>{{ c }}</td>
                <td>{{ value('channel', 'scheduled', c) }}</td>
                <td>{{ value('channel', 'attempted', c) }}</td>
                <td>{{ value('channel', 'ACCEPTED', c) }}</td>
                <td>{{ value('channel', 'BLOCKED', c) }}</td>
                <td>{{ value('channel', 'SKIPPED', c) + value('channel', 'CANCELLED', c) }}</td>
                <td>{{ value('channel', 'FAILED', c) }}</td>
                <td>{{ value('channel', 'UNKNOWN', c) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>
          Acceptance is not confirmed delivery. Retry attempts do not increase logical
          scheduled/attempted counts. Open/click tracking is not enabled.
        </p>
      </section>
      <section class="panel">
        <h2>Node funnel · published v{{ funnelVersion }} · {{ funnelEntries }} entries</h2>
        <p v-if="!selected">Select one journey to inspect its node funnel.</p>
        <div v-else class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Node</th>
                <th>Entered</th>
                <th>Succeeded</th>
                <th>Skipped</th>
                <th>Failed</th>
                <th>Drop-off from entry</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="n in funnel" :key="n.node_id">
                <td>{{ n.node_id }}</td>
                <td>{{ n.value }}</td>
                <td>{{ nodeValue(n.node_id, 'SUCCEEDED') }}</td>
                <td>{{ nodeValue(n.node_id, 'SKIPPED') }}</td>
                <td>{{ nodeValue(n.node_id, 'FAILED') }}</td>
                <td>{{ funnelEntries ? ((1 - n.value / funnelEntries) * 100).toFixed(1) : 0 }}%</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>Node success means the step completed; message acceptance is reported separately.</p>
      </section>
      <section class="panel">
        <h2>Exit reasons</h2>
        <p v-for="m in summary.metrics.filter((m) => m.dimension === 'exit_reason')" :key="m.key">
          {{ m.key }} · {{ m.value }}
        </p>
      </section>
      <section v-if="!journeyId && !overview" class="panel">
        <h2>Journey comparison</h2>
        <p>Lifetime logical entries and conversions. Latest 100 journeys.</p>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Journey</th>
                <th>Entries</th>
                <th>Active</th>
                <th>Converted / entries</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="j in comparison" :key="j.id">
                <td>
                  <button
                    @click="
                      selected = j.id;
                      load();
                    "
                  >
                    {{ j.name }}
                  </button>
                </td>
                <td>{{ j.entries }}</td>
                <td>{{ j.active }}</td>
                <td>
                  {{ j.converted }} / {{ j.entries }} ·
                  {{ j.entries ? ((100 * j.converted) / j.entries).toFixed(1) : 0 }}%
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <button v-if="selected" @click="emit('openJourney', selected)">
          Open selected journey
        </button>
      </section>
      <section v-if="overview && activity" class="panel">
        <h2>Recent executions &amp; system alerts</h2>
        <p v-if="!activity.failed_executions.length">
          No failed executions in the current error list.
        </p>
        <button
          v-for="e in activity.failed_executions"
          :key="e.id"
          class="list-row"
          @click="emit('openJourney', e.journey_id)"
        >
          Failed: {{ e.customer_id }} · {{ e.id }}</button
        ><button
          v-for="e in activity.recent_executions"
          :key="e.id"
          class="list-row"
          @click="emit('openJourney', e.journey_id)"
        >
          {{ e.customer_id }} · {{ e.state }} · {{ e.id }}
        </button>
      </section></template
    >
  </section>
</template>
<style scoped>
.table-scroll {
  overflow: auto;
}
table {
  width: 100%;
  text-align: left;
}
th,
td {
  padding: 12px;
}
.trend-row {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 12px 0;
  flex-wrap: wrap;
}
.trend-bar {
  background: #88a77b;
  height: 16px;
  min-width: 2px;
  border-radius: 4px;
}
</style>
