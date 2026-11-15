<script setup lang="ts">
import SegmentedControl from './ui/SegmentedControl.vue';
import EmptyState from './ui/EmptyState.vue';
import { errorMessage, plural } from '../lib/format';
import { computed, onMounted, ref } from 'vue';
import { api, type Journey } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import MetricCard from './ui/MetricCard.vue';
import StatusBadge from './ui/StatusBadge.vue';
import TrendChart from './ui/TrendChart.vue';
const props = defineProps<{ name: string; journeys: Journey[]; readonly?: boolean }>();
const emit = defineEmits<{
  create: [];
  templates: [];
  open: [string];
  browse: [string];
  analytics: [];
}>();
type Metric = { dimension: string; key: string; metric: string; value: number };
type Daily = { day: string; dimension: string; key: string; metric: string; value: number };
type Activity = {
  id: string;
  journey_id: string;
  customer_id: string;
  state: string;
  started_at?: string;
  updated_at?: string;
};
const metrics = ref<Metric[]>([]);
const daily = ref<Daily[]>([]);
const recent = ref<Activity[]>([]);
const failed = ref<Activity[]>([]);
const updated = ref('');
const entries24 = ref(0);
const days = ref(30);
const busy = ref(true);
const error = ref('');
const value = (dim: string, metric: string, key = '') =>
  metrics.value.find((m) => m.dimension === dim && m.metric === metric && m.key === key)?.value ||
  0;
const entries = computed(() => value('execution', 'entries'));
const conversion = computed(() =>
  entries.value ? ((value('execution', 'converted') / entries.value) * 100).toFixed(1) : '0.0',
);
const chartData = computed(() => {
  const result = [];
  const now = new Date();
  const end = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  for (let i = days.value - 1; i >= 0; i--) {
    const day = new Date(end - i * 86400000).toISOString().slice(0, 10);
    result.push({
      day,
      entries:
        daily.value.find(
          (d) => d.day === day && d.dimension === 'execution' && d.metric === 'entries',
        )?.value || 0,
      converted:
        daily.value.find(
          (d) => d.day === day && d.dimension === 'execution' && d.metric === 'converted',
        )?.value || 0,
    });
  }
  return result;
});
const greeting = computed(() => {
  const h = new Date().getHours();
  return h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
});
const journeyName = (id: string) => props.journeys.find((j) => j.id === id)?.name || 'Journey';
const shortTime = (s?: string) =>
  s ? new Date(s).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '—';
async function load() {
  busy.value = true;
  error.value = '';
  try {
    const [summary, trend, activity, lastDay] = await Promise.all([
      api<{ metrics: Metric[]; updated_at: string }>('/overview'),
      api<{ daily: Daily[] }>(`/analytics?days=${days.value}`),
      api<{ recent_executions: Activity[]; failed_executions: Activity[] }>('/overview/activity'),
      api<{ entries: number }>('/overview/entries-24h'),
    ]);
    metrics.value = summary.metrics;
    updated.value = summary.updated_at;
    daily.value = trend.daily;
    recent.value = activity.recent_executions;
    failed.value = activity.failed_executions;
    entries24.value = lastDay.entries;
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
onMounted(() => void load());
</script>
<template>
  <section class="overview-dashboard" :aria-busy="busy">
    <div class="page-heading">
      <div>
        <p class="eyebrow">Workspace overview</p>
        <h1>{{ greeting }}, {{ name.split(' ')[0] }}<span class="greeting-dot">.</span></h1>
        <p class="page-description">Manage your journeys and follow every customer moment.</p>
      </div>
      <div class="heading-actions">
        <button @click="emit('templates')"><AppIcon name="templates" />Browse templates</button
        ><button v-if="!readonly" class="primary" @click="emit('create')">
          <AppIcon name="plus" />Create journey
        </button>
      </div>
    </div>
    <p v-if="error" class="error-message" role="alert">
      {{ error }} <button @click="load">Retry</button>
    </p>
    <div class="overview-metrics">
      <MetricCard
        label="Active journeys"
        :value="value('journey', 'Active')"
        :caption="`${plural(value('journey', 'total'), 'journey')} in your workspace`"
        icon="workflow"
        @select="emit('browse', 'Active')"
      /><MetricCard
        label="Active executions"
        :value="value('execution', 'RUNNING') + value('execution', 'WAITING')"
        :caption="`${entries24} journey entries in the last 24 hours`"
        icon="users"
        tone="blue"
        @select="emit('browse', '')"
      /><MetricCard
        label="Messages accepted"
        :value="value('channel', 'ACCEPTED', 'email') + value('channel', 'ACCEPTED', 'sms')"
        caption="Email and SMS · provider acceptance"
        icon="email"
        tone="green"
        @select="emit('analytics')"
      /><MetricCard
        label="Conversion rate"
        :value="conversion + '%'"
        :caption="`${value('execution', 'converted')} converted / ${entries} entries · all time`"
        icon="chart"
        tone="amber"
        @select="emit('analytics')"
      />
    </div>
    <div class="overview-middle">
      <section class="surface activity-chart">
        <div class="surface-heading">
          <div>
            <h2>Journey activity</h2>
            <p>Entries and conversions by entry date</p>
          </div>
          <SegmentedControl
            v-model="days"
            class="period-segments"
            label="Activity period"
            hide-label
            :options="[
              { value: 7, label: '7 days' },
              { value: 30, label: '30 days' },
              { value: 90, label: '90 days' },
            ]"
            @change="load"
          />
        </div>
        <div class="chart-legend">
          <span><i class="legend-dot violet" />Journey entries</span
          ><span><i class="legend-dot green" />Conversions</span
          ><span class="chart-timezone">UTC</span>
        </div>
        <TrendChart :data="chartData" />
      </section>
      <section class="surface channel-overview">
        <div class="surface-heading">
          <h2>Channel performance</h2>
          <button
            class="icon-button"
            aria-label="Open channel analytics"
            @click="emit('analytics')"
          >
            <AppIcon name="arrow" />
          </button>
        </div>
        <div v-for="c in ['email', 'sms']" :key="c" class="channel-summary">
          <div class="channel-summary-title">
            <span class="channel-icon" :class="c"
              ><AppIcon :name="c === 'email' ? 'email' : 'message'"
            /></span>
            <div>
              <strong>{{ c === 'email' ? 'Email' : 'SMS' }}</strong
              ><small>{{ value('channel', 'scheduled', c) }} logical message plans</small>
            </div>
            <b>{{ value('channel', 'ACCEPTED', c) }}</b>
          </div>
          <div class="channel-progress">
            <span
              :style="{
                width:
                  (value('channel', 'ACCEPTED', c) /
                    Math.max(1, value('channel', 'scheduled', c))) *
                    100 +
                  '%',
              }"
            />
          </div>
          <div class="channel-summary-meta">
            <span>{{ value('channel', 'BLOCKED', c) }} blocked</span
            ><span>{{ value('channel', 'FAILED', c) }} failed</span
            ><span>{{ value('channel', 'UNKNOWN', c) }} unknown</span>
          </div>
        </div>
        <p class="card-footnote">
          <AppIcon name="shield" :size="14" />Provider accepted does not mean delivered.
        </p>
      </section>
    </div>
    <div class="overview-bottom">
      <section class="surface recent-journeys">
        <div class="surface-heading">
          <div>
            <h2>
              Your journeys <span class="count-label">{{ value('journey', 'total') }}</span>
            </h2>
            <p>Pick up where you left off</p>
          </div>
          <button class="text-button" @click="emit('browse', '')">
            View all <AppIcon name="arrow" :size="15" />
          </button>
        </div>
        <div class="status-filters">
          <button
            v-for="s in ['All', 'Active', 'Draft', 'Paused']"
            :key="s"
            :class="{ active: s === 'All' }"
            @click="emit('browse', s === 'All' ? '' : s)"
          >
            {{ s }} <span>{{ value('journey', s === 'All' ? 'total' : s) }}</span>
          </button>
        </div>
        <EmptyState v-if="!journeys.length" icon="workflow" title="Your first journey starts here">
          Create a workflow from scratch or begin with a template.
          <template v-if="!readonly" #action>
            <button class="primary" @click="emit('create')">
              <AppIcon name="plus" />Create journey
            </button>
          </template>
        </EmptyState>
        <div v-for="j in journeys.slice(0, 5)" :key="j.id" class="recent-journey-row">
          <span class="journey-type-icon"><AppIcon name="workflow" :size="19" /></span
          ><button class="journey-name-button" @click="emit('open', j.id)">
            <strong>{{ j.name }}</strong
            ><small>{{ j.description || 'No description yet' }}</small></button
          ><StatusBadge :status="j.status" /><span class="row-chevron"
            ><AppIcon name="arrow" :size="16"
          /></span>
        </div>
      </section>
      <section class="surface recent-activity">
        <div class="surface-heading">
          <h2>Recent activity</h2>
          <span class="live-label"><i />Recent entries</span>
        </div>
        <EmptyState v-if="!recent.length" icon="events" title="A little quiet for now" compact>
          Customer entries and journey outcomes will appear here.
        </EmptyState>
        <button
          v-for="e in recent.slice(0, 5)"
          :key="e.id"
          class="activity-row"
          @click="emit('open', e.journey_id)"
        >
          <span class="activity-marker" :class="e.state === 'FAILED' ? 'error' : ''"
            ><AppIcon :name="e.state === 'COMPLETED' ? 'check' : 'zap'" :size="13" /></span
          ><span
            ><strong>{{ e.customer_id }}</strong
            ><small>{{ journeyName(e.journey_id) }} · {{ e.state.toLowerCase() }}</small></span
          ><time>{{ shortTime(e.started_at) }}</time>
        </button>
        <div v-if="failed.length" class="inline-alert">
          <AppIcon name="events" :size="16" />{{ failed.length }} recent failed executions need
          attention.
        </div>
        <div class="activity-freshness">
          <AppIcon name="refresh" :size="12" />{{
            updated ? 'Updated ' + shortTime(updated) : 'Waiting for the metrics worker'
          }}<button class="text-button" :disabled="busy" @click="load">Refresh</button>
        </div>
      </section>
    </div>
    <section class="getting-started-strip">
      <span class="getting-started-icon"><AppIcon name="flask" :size="23" /></span>
      <div>
        <strong>Build with confidence. Test before you publish.</strong>
        <p>Try different customer profiles and fast-forward time in Test Lab.</p>
      </div>
      <button @click="emit('browse', '')">
        Explore journeys <AppIcon name="arrow" :size="15" />
      </button>
    </section>
  </section>
</template>
