<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api, type Journey } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import MetricCard from './ui/MetricCard.vue';
import TrendChart from './ui/TrendChart.vue';
import FilterSelect from './ui/FilterSelect.vue';
import PeriodSelect from './ui/PeriodSelect.vue';
const props = defineProps<{ journeyId?: string; overview?: boolean }>();
type Metric = { dimension: string; key: string; metric: string; value: number };
type Summary = { metrics: Metric[]; updated_at?: string; semantics: string };
type Daily = { day: string; dimension: string; key: string; metric: string; value: number };
const summary = ref<Summary>();
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
function at(day: string, metric: string) {
  return (
    daily.value?.daily.find(
      (d) => d.day === day && d.dimension === 'execution' && d.metric === metric,
    )?.value || 0
  );
}
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
const chartData = computed(() => {
  const end = new Date();
  const midnight = Date.UTC(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate());
  return Array.from({ length: days.value }, (_, i) => {
    const day = new Date(midnight - (days.value - 1 - i) * 86400000).toISOString().slice(0, 10);
    return { day, entries: at(day, 'entries'), converted: at(day, 'converted') };
  });
});
const nodeLabels = ref<Record<string, string>>({});
const labelCache = new Map<string, Record<string, string>>();
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
      if (funnelVersion.value) {
        const key = selected.value + ':' + funnelVersion.value;
        if (!labelCache.has(key)) {
          const result = await api<{ workflow: { nodes: { id: string; label: string }[] } }>(
            `/journeys/${selected.value}/versions/${funnelVersion.value}`,
          );
          if (labelCache.size >= 10) labelCache.clear();
          labelCache.set(
            key,
            Object.fromEntries(result.workflow.nodes.map((n) => [n.id, n.label])),
          );
        }
        nodeLabels.value = labelCache.get(key) || {};
      }
      nodeMetrics.value = funnelVersion.value
        ? await api(`/journeys/${selected.value}/node-stats?version=${funnelVersion.value}`)
        : [];
    } else {
      comparison.value = await api<typeof comparison.value>('/analytics/comparison');
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
  <section class="analytics-page" :aria-busy="busy">
    <div class="page-heading">
      <div>
        <h1>{{ journeyId ? 'Journey analytics' : 'Analytics' }}</h1>
        <p class="page-description">Understand journey performance and channel outcomes.</p>
      </div>
      <div class="heading-actions">
        <button :disabled="busy" @click="load">
          <AppIcon name="refresh" :size="15" />Refresh metrics
        </button>
      </div>
    </div>
    <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
    <div class="analytics-filter-bar">
      <FilterSelect
        v-if="!journeyId"
        v-model="selected"
        label="Journey"
        icon="workflow"
        searchable
        search-placeholder="Find a journey…"
        :disabled="busy"
        :options="[
          { value: '', label: 'All journeys', description: 'Workspace-wide performance' },
          ...journeys.map((j) => ({
            value: j.id,
            label: j.name,
            description: j.status + (j.version ? ' · v' + j.version : ''),
          })),
        ]"
        @update:model-value="load"
      />
      <div class="analytics-scope-note">
        <AppIcon name="chart" :size="15" /><span
          >Performance overview<small>All-time totals</small></span
        >
      </div>
    </div>
    <div class="overview-metrics">
      <MetricCard
        label="Journey entries"
        :value="entries"
        caption="Logical enrollments · all time"
        icon="workflow"
        @select="emit('browseJourneys', '')"
      /><MetricCard
        label="Unique customers"
        :value="value('execution', 'unique_customers')"
        caption="Distinct enrolled customers · all time"
        icon="users"
        tone="blue"
        @select="emit('navigate', 'Customers')"
      /><MetricCard
        label="Active executions"
        :value="value('execution', 'RUNNING') + value('execution', 'WAITING')"
        :caption="value('execution', 'COMPLETED') + ' completed executions'"
        icon="events"
        tone="green"
        @select="emit('browseJourneys', '')"
      /><MetricCard
        label="Conversion rate"
        :value="rate + '%'"
        :caption="converted + ' converted / ' + entries + ' entries'"
        icon="chart"
        tone="amber"
        @select="emit('browseJourneys', '')"
      />
    </div>
    <section class="surface analytics-trend">
      <div class="surface-heading">
        <div>
          <h2>Entry &amp; conversion trend</h2>
          <p>Outcomes are attributed to the day customers entered the journey.</p>
        </div>
        <PeriodSelect v-model="days" :disabled="busy" @update:model-value="load" />
      </div>
      <div class="chart-legend">
        <span><i class="legend-dot violet" />Journey entries</span
        ><span><i class="legend-dot green" />Conversions</span
        ><span class="chart-timezone">UTC</span>
      </div>
      <TrendChart :data="chartData" />
    </section>
    <section class="surface analytics-channels">
      <div class="surface-heading">
        <div>
          <h2>Channel performance · logical sends</h2>
          <p>Retries are grouped under the original message.</p>
        </div>
      </div>
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Channel</th>
              <th>Planned</th>
              <th>Attempted</th>
              <th>Provider accepted</th>
              <th>Blocked</th>
              <th>Skipped</th>
              <th>Failed</th>
              <th>Unknown</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in ['email', 'sms']" :key="c">
              <td>
                <span class="channel-cell"
                  ><span class="channel-icon" :class="c"
                    ><AppIcon :name="c === 'email' ? 'email' : 'message'" :size="16" /></span
                  >{{ c === 'email' ? 'Email' : 'SMS' }}</span
                >
              </td>
              <td>{{ value('channel', 'scheduled', c) }}</td>
              <td>{{ value('channel', 'attempted', c) }}</td>
              <td>
                <span class="accepted-number">{{ value('channel', 'ACCEPTED', c) }}</span>
              </td>
              <td>{{ value('channel', 'BLOCKED', c) }}</td>
              <td>{{ value('channel', 'SKIPPED', c) + value('channel', 'CANCELLED', c) }}</td>
              <td>{{ value('channel', 'FAILED', c) }}</td>
              <td>{{ value('channel', 'UNKNOWN', c) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="table-caption">
        <AppIcon name="shield" :size="14" />Provider acceptance is not confirmed delivery.
        Open/click tracking is not enabled.
      </p>
    </section>
    <div class="analytics-bottom">
      <section class="surface">
        <div class="surface-heading">
          <div>
            <h2>Node funnel</h2>
            <p>
              {{
                selected
                  ? 'Published v' + funnelVersion + ' · ' + funnelEntries + ' entries'
                  : 'Choose a journey to inspect its path'
              }}
            </p>
          </div>
        </div>
        <div v-if="selected && funnel.length" class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Step</th>
                <th>Entered</th>
                <th>Completed</th>
                <th>Skipped</th>
                <th>Drop-off</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="n in funnel" :key="n.node_id">
                <td>{{ nodeLabels[n.node_id] || n.node_id }}</td>
                <td>{{ n.value }}</td>
                <td>{{ nodeValue(n.node_id, 'SUCCEEDED') }}</td>
                <td>{{ nodeValue(n.node_id, 'SKIPPED') }}</td>
                <td>{{ funnelEntries ? ((1 - n.value / funnelEntries) * 100).toFixed(1) : 0 }}%</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-state">
          <span class="empty-icon"><AppIcon name="branch" :size="26" /></span>
          <h3>No funnel data yet</h3>
          <p>
            {{
              selected
                ? 'Node activity will appear after customers enter.'
                : 'Select a journey above to see step-level performance.'
            }}
          </p>
        </div>
      </section>
      <section class="surface">
        <div class="surface-heading"><h2>Journey outcomes</h2></div>
        <div class="outcomes-summary">
          <div>
            <span>Average terminal duration</span
            ><strong
              >{{
                (
                  value('execution', 'duration_ms') /
                  Math.max(1, value('execution', 'finished_count')) /
                  1000
                ).toFixed(1)
              }}<small>sec</small></strong
            >
          </div>
          <div
            v-for="m in summary?.metrics.filter(
              (m) => m.dimension === 'exit_reason' && m.value > 0,
            ) || []"
            :key="m.key"
            class="outcome-row"
          >
            <span>{{ m.key.replaceAll('_', ' ') }}</span
            ><strong>{{ m.value }}</strong>
          </div>
          <p v-if="!summary?.metrics.some((m) => m.dimension === 'exit_reason' && m.value > 0)">
            Exit reasons will appear as journeys finish.
          </p>
        </div>
      </section>
    </div>
    <section v-if="!journeyId" class="surface">
      <div class="surface-heading">
        <div>
          <h2>Journey comparison</h2>
          <p>Latest 100 journeys · lifetime logical entries</p>
        </div>
      </div>
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Journey</th>
              <th>Entries</th>
              <th>Active</th>
              <th>Converted</th>
              <th>Conversion rate</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="j in comparison" :key="j.id">
              <td>
                <button class="journey-name-button" @click="emit('openJourney', j.id)">
                  {{ j.name }}
                </button>
              </td>
              <td>{{ j.entries }}</td>
              <td>{{ j.active }}</td>
              <td>{{ j.converted }}</td>
              <td>{{ j.entries ? ((100 * j.converted) / j.entries).toFixed(1) : 0 }}%</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <details class="metric-definitions">
      <summary><AppIcon name="file" :size="14" />Metric definitions</summary>
      <p>{{ summary?.semantics }}</p>
      <p>{{ daily?.semantics }}</p>
      <p>
        Step completion and message acceptance are separate outcomes. Average terminal duration
        includes all exit reasons.
      </p>
    </details>
  </section>
</template>
