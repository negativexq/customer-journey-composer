<script setup lang="ts">
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { errorMessage } from '../lib/format';
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  onActivated,
  onDeactivated,
  ref,
  watch,
} from 'vue';
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'reka-ui';
import JourneyMetricsCanvas from './JourneyMetricsCanvas.vue';
import ExperimentResults from './ExperimentResults.vue';
import AudienceEntry from './AudienceEntry.vue';
import PathFlow from './PathFlow.vue';
import ScopeSelect from './ui/ScopeSelect.vue';
import MetricCard from './ui/MetricCard.vue';
import TrendChart from './ui/TrendChart.vue';
import { portLabel } from '../lib/workflow-presentation';
import { metric, reasonLabel, type Insights } from '../lib/insights';
import { api } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import type { DeliveryEngagement } from '../lib/engagement';
import type { Workflow } from '../lib/workflow';
const props = defineProps<{
  journeyId: string;
  mode?: 'live' | 'analytics';
  readonly?: boolean;
  admin?: boolean;
  initialExecutionId?: string;
  initialDeliveryId?: string;
}>();
const emit = defineEmits<{
  selection: [{ executionID?: string; deliveryID?: string }];
  inspect: [{ kind: 'event' | 'execution' | 'message'; id: string }];
}>();
type Execution = {
  id: string;
  version_id: string;
  journey_id: string;
  entry_event_id: string;
  customer_id: string;
  state: string;
  current_node_id: string;
  started_at: string;
  exit_reason?: string;
  steps: {
    node_id: string;
    state: string;
    output: { port?: string; condition?: boolean; exit_reason?: string; error?: string };
    started_at: string;
    completed_at?: string;
  }[];
  jobs: {
    id: string;
    node_id: string;
    kind: string;
    state: string;
    due_at: string;
    attempts: number;
  }[];
  waiters: { node_id: string; event_name: string; state: string; deadline: string }[];
  deliveries: {
    id: string;
    node_id: string;
    channel: string;
    status: string;
    decision: unknown;
    attempts: number;
    last_error?: string;
  }[];
};
const versions = ref<{ id: string; version: number }[]>([]);
const scopeKey = 'journey-observe:' + props.journeyId;
function remembered(): { version?: number; liveDays?: number; analyticsDays?: number } {
  try {
    return JSON.parse(sessionStorage.getItem(scopeKey) || '{}');
  } catch {
    return {};
  }
}
const preferences = remembered();
const version = ref(preferences.version || 0);
const workflow = ref<Workflow>();
const insights = ref<Insights>();
const days = ref(
  props.mode === 'analytics' ? (preferences.analyticsDays ?? 30) : (preferences.liveDays ?? 0),
);
watch([version, days], () => {
  try {
    sessionStorage.setItem(
      scopeKey,
      JSON.stringify({
        ...remembered(),
        version: version.value,
        [props.mode === 'analytics' ? 'analyticsDays' : 'liveDays']: days.value,
      }),
    );
  } catch {
    /* Optional session preference. */
  }
});
const selectedNode = ref(''),
  branch = ref(''),
  exitFilter = ref(''),
  nodeCurrent = ref(false),
  nodeTab = ref('summary');
const canvas = ref<InstanceType<typeof JourneyMetricsCanvas>>();
const showMessages = ref(false),
  overdueFilter = ref(false);
const expanded = ref(false),
  hasMore = ref(false),
  paging = ref(false);
const pageSize = 25;
const messages = ref<
  {
    id: string;
    execution_id: string;
    customer_id: string;
    node_id: string;
    channel: string;
    status: string;
    decision: unknown;
    attempts: number;
    created_at: string;
  }[]
>([]);
const messageStatus = ref(''),
  messageChannel = ref(''),
  messageReason = ref('');
const messageMore = ref(false),
  messageBusy = ref(false);
let messageRequest = 0;
const selectedStep = computed(() => workflow.value?.nodes.find((n) => n.id === selectedNode.value));
const value = (dimension: string, name: string, key = '') =>
  metric(insights.value, dimension, name, key);
const nodeValue = (name: string, dimension = 'node') => value(dimension, name, selectedNode.value);
const nodeReasons = computed(
  () =>
    insights.value?.metrics.filter(
      (m) => m.dimension === 'node_reason' && m.key === selectedNode.value && m.value > 0,
    ) || [],
);
const exitReasons = computed(
  () => insights.value?.metrics.filter((m) => m.dimension === 'exit_reason' && m.value > 0) || [],
);
const chartData = computed(() => {
  const daily = insights.value?.daily || [];
  const end = insights.value?.scope.until ? Date.parse(insights.value.scope.until) : Date.now();
  const first = daily.length
    ? Date.parse([...daily].sort((a, b) => a.day.localeCompare(b.day))[0]!.day + 'T00:00:00Z')
    : end - 30 * 86400000;
  const length = days.value || Math.max(1, Math.ceil((end - first) / 86400000));
  if (length > 90) {
    const months = new Map<string, { day: string; entries: number; converted: number }>();
    for (const d of daily) {
      const key = d.day.slice(0, 7) + '-01';
      const item = months.get(key) || { day: key, entries: 0, converted: 0 };
      if (d.metric === 'entries') item.entries += d.value;
      else if (d.metric === 'converted') item.converted += d.value;
      months.set(key, item);
    }
    return [...months.values()].sort((a, b) => a.day.localeCompare(b.day));
  }
  return Array.from({ length }, (_, i) => {
    const day = new Date(end - (length - i) * 86400000).toISOString().slice(0, 10);
    return {
      day,
      entries: daily.find((d) => d.day === day && d.metric === 'entries')?.value || 0,
      converted: daily.find((d) => d.day === day && d.metric === 'converted')?.value || 0,
    };
  });
});
const graphOrder = computed(() => {
  if (!workflow.value) return [];
  const w = workflow.value;
  const indegree = new Map(w.nodes.map((n) => [n.id, 0]));
  for (const e of w.edges) indegree.set(e.target, (indegree.get(e.target) || 0) + 1);
  const queue = w.nodes.filter((n) => !indegree.get(n.id));
  const result: Workflow['nodes'] = [];
  while (queue.length) {
    const n = queue.shift()!;
    result.push(n);
    for (const e of w.edges.filter((e) => e.source === n.id)) {
      indegree.set(e.target, indegree.get(e.target)! - 1);
      if (!indegree.get(e.target)) {
        const next = w.nodes.find((n) => n.id === e.target);
        if (next) queue.push(next);
      }
    }
  }
  return result;
});
const freshness = computed(() => {
  const d = insights.value;
  if (!d) return 'Loading metrics…';
  if (!d.coverage_complete) return 'Preparing historical breakdowns';
  if (!d.projector_last_success || Date.now() - Date.parse(d.projector_last_success) > 15000)
    return 'Counters may be delayed';
  if (d.queue_oldest_at && Date.now() - Date.parse(d.queue_oldest_at) > 10000)
    return 'Counters catching up';
  return 'Projection healthy';
});
function scopedParams() {
  const p = new URLSearchParams({ version: String(version.value), limit: String(pageSize) });
  if (insights.value?.scope.from) p.set('from', insights.value.scope.from);
  if (insights.value?.scope.until) p.set('until', insights.value.scope.until);
  return p;
}
function executionParams() {
  const p = scopedParams();
  if (customer.value) p.set('customer_id', customer.value);
  if (filter.value === 'active') p.set('active', 'true');
  else if (filter.value) p.set('state', filter.value);
  if (selectedNode.value) p.set('node_id', selectedNode.value);
  if (branch.value) p.set('port', branch.value);
  if (exitFilter.value) p.set('exit_reason', exitFilter.value);
  if (nodeCurrent.value) p.set('current', 'true');
  if (overdueFilter.value) p.set('overdue', 'true');
  return p;
}
async function loadMore() {
  if (!executions.value.length || paging.value) return;
  paging.value = true;
  const expected = executionParams().toString();
  const last = executions.value.at(-1)!;
  const params = executionParams();
  params.set('before', last.started_at);
  params.set('before_id', last.id);
  try {
    const rows = await api<Execution[]>(`/journeys/${props.journeyId}/executions?${params}`);
    if (expected !== executionParams().toString()) return;
    executions.value.push(...rows);
    hasMore.value = rows.length === pageSize;
    expanded.value = true;
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    paging.value = false;
  }
}
async function loadMessages(more = false) {
  const ticket = ++messageRequest;
  messageBusy.value = true;
  const params = scopedParams();
  if (selectedNode.value) params.set('node_id', selectedNode.value);
  if (messageStatus.value) params.set('status', messageStatus.value);
  if (messageChannel.value) params.set('channel', messageChannel.value);
  if (messageReason.value) params.set('reason', messageReason.value);
  if (more && messages.value.length) {
    const last = messages.value.at(-1)!;
    params.set('before', last.created_at);
    params.set('before_id', last.id);
  }
  try {
    const rows = await api<typeof messages.value>(
      `/journeys/${props.journeyId}/messages?${params}`,
    );
    if (ticket !== messageRequest) return;
    messages.value = more ? [...messages.value, ...rows] : rows;
    messageMore.value = rows.length === pageSize;
  } catch (cause) {
    if (ticket === messageRequest) error.value = errorMessage(cause);
  } finally {
    if (ticket === messageRequest) messageBusy.value = false;
  }
}
function clearSelection() {
  tracking.value = false;
  selectionRequest++;
  deliveryRequest++;
  selected.value = undefined;
  delivery.value = undefined;
  emit('selection', {});
}
function selectNode(id: string) {
  if (sectioned.value && section.value === 'messages') section.value = 'map';
  showMessages.value = false;
  overdueFilter.value = false;
  clearSelection();
  selectedNode.value = id;
  branch.value = '';
  exitFilter.value = '';
  nodeCurrent.value = false;
  nodeTab.value = 'summary';
  messageStatus.value = '';
  messageReason.value = '';
  messageChannel.value = '';
  void refresh(true);
}
// Live mode splits the monitor into sections, like Settings: the map, the people in the journey
// and the messages it sent. Analytics keeps everything on one page.
type Section = 'map' | 'people' | 'messages';
const section = ref<Section>(
  props.initialExecutionId || props.initialDeliveryId ? 'people' : 'map',
);
const sectioned = computed(() => props.mode !== 'analytics');
const shows = (part: Section) => !sectioned.value || section.value === part;
function openSection(next: Section) {
  if (next === section.value) return;
  section.value = next;
  if (next === 'messages') browseChannel('');
  else if (showMessages.value) {
    showMessages.value = false;
    if (!selectedNode.value) void refresh(true);
  }
}
// A count and the list it filters can be a screen apart. Bring the list to the
// reader only when it is out of view, so clicks next to the list do not jump.
const people = ref<HTMLElement>();
async function revealPeople() {
  if (sectioned.value) section.value = 'people';
  await nextTick();
  const list = people.value;
  if (!list) return;
  const top = list.getBoundingClientRect().top;
  if (top >= 0 && top <= window.innerHeight * 0.6) return;
  list.scrollIntoView({
    block: 'start',
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
  });
}
function browse(state = '', reason = '') {
  showMessages.value = false;
  overdueFilter.value = false;
  clearSelection();
  selectedNode.value = '';
  branch.value = '';
  exitFilter.value = reason;
  filter.value = state;
  nodeCurrent.value = false;
  void refresh(true);
  if (state || reason) void revealPeople();
}
function branchPeople(port: string) {
  branch.value = port;
  filter.value = '';
  exitFilter.value = '';
  nodeCurrent.value = false;
  nodeTab.value = 'people';
  void refresh(true);
  void revealPeople();
}
// From an experiment result to the people on one variant of that split.
function variantPeople(node: string, port: string) {
  showMessages.value = false;
  selectedNode.value = node;
  branchPeople(port);
}
// From a step count to the people behind it: everyone who reached the step, or
// only those waiting there now.
function stepPeople(id: string, waiting = false) {
  showMessages.value = false;
  overdueFilter.value = false;
  clearSelection();
  selectedNode.value = id;
  branch.value = '';
  exitFilter.value = '';
  nodeCurrent.value = waiting;
  filter.value = waiting ? 'WAITING' : '';
  nodeTab.value = 'people';
  canvas.value?.focus(id);
  void refresh(true);
  void revealPeople();
}
const breakdown = computed(() =>
  [
    selectedStep.value
      ? (nodeCurrent.value ? 'Currently at ' : 'Reached ') + selectedStep.value.label
      : '',
    branch.value ? 'Branch: ' + portLabel(branch.value) : '',
    exitFilter.value ? 'Outcome: ' + reasonLabel(exitFilter.value) : '',
    overdueFilter.value ? 'Waits past due' : '',
  ].filter(Boolean),
);
function reasonMessages(reason: string) {
  messageReason.value = reason;
  messageStatus.value = '';
  nodeTab.value = 'messages';
  void loadMessages();
}
async function changePeriod() {
  clearSelection();
  insights.value = undefined;
  await refresh(true);
}
function browseChannel(channel: string, status = '') {
  clearSelection();
  selectedNode.value = '';
  showMessages.value = true;
  if (sectioned.value) section.value = 'messages';
  messageChannel.value = channel;
  messageStatus.value = status;
  messageReason.value = '';
  nodeTab.value = 'messages';
  void loadMessages();
}
function browseOverdue() {
  browse('WAITING');
  overdueFilter.value = true;
  void refresh(true);
  void revealPeople();
}
async function changeScope() {
  insights.value = undefined;
  showMessages.value = false;
  overdueFilter.value = false;
  clearSelection();
  selectedNode.value = '';
  branch.value = '';
  exitFilter.value = '';
  filter.value = '';
  await graph();
  await refresh(true);
}
watch(nodeTab, (t) => {
  if (t === 'messages') void loadMessages();
});
const customer = ref('');
const filter = ref('');
const executions = ref<Execution[]>([]);
const selected = ref<Execution>();
const error = ref('');
const busy = ref(false);
const poll = ref(props.mode !== 'analytics');
const delivery = ref<{
  id: string;
  provider: string;
  execution_id?: string;
  status: string;
  decision: unknown;
  attempt_history: unknown[];
  engagement?: DeliveryEngagement[];
  last_error?: string;
}>();
const stepLabel = (id: string) => workflow.value?.nodes.find((node) => node.id === id)?.label || id;
const time = (at: string) => new Date(at).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC';
function decisionText(output: Execution['steps'][number]['output']) {
  if (output.error) return output.error;
  if (output.condition !== undefined)
    return output.condition
      ? 'Condition matched · Yes branch'
      : 'Condition did not match · No branch';
  if (output.exit_reason) return 'Exit: ' + output.exit_reason.replaceAll('_', ' ');
  return output.port ? 'Continued via ' + output.port : 'Waiting for this step to finish';
}
const stamp = ref('');
let timer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;
let inactive = false;
let refreshQueued = false;
let selectionRequest = 0;
let routeRestore = 0;
let deliveryRequest = 0;
const tracking = ref(false);
async function graph() {
  const requested = version.value;
  const result = await api<{ workflow: Workflow }>(
    `/journeys/${props.journeyId}/versions/${requested}`,
  );
  if (!disposed && requested === version.value) workflow.value = result.workflow;
}
async function refresh(reset = false) {
  if (reset) expanded.value = false;
  if (disposed || inactive) return;
  if (busy.value) {
    refreshQueued = true;
    return;
  }
  busy.value = true;
  error.value = '';
  try {
    if (reset || !versions.value.length) {
      const previousVersion = version.value;
      versions.value = await api(`/journeys/${props.journeyId}/versions`);
      if (versions.value.length) {
        if (!versions.value.some((v) => v.version === version.value))
          version.value = versions.value[0]!.version;
        if (!workflow.value || previousVersion !== version.value) await graph();
      }
    }
    if (!versions.value.length) {
      executions.value = [];
      return;
    }
    const expected = JSON.stringify([
      customer.value,
      filter.value,
      version.value,
      days.value,
      selectedNode.value,
      branch.value,
      exitFilter.value,
      nodeCurrent.value,
    ]);
    const counts = await api<Insights>(
      `/journeys/${props.journeyId}/insights?version=${version.value}&days=${days.value}&include_unique=${props.mode === 'analytics'}`,
    );
    if (disposed || inactive) return;
    if (
      expected !==
      JSON.stringify([
        customer.value,
        filter.value,
        version.value,
        days.value,
        selectedNode.value,
        branch.value,
        exitFilter.value,
        nodeCurrent.value,
      ])
    ) {
      refreshQueued = true;
      return;
    }
    insights.value = counts;
    if (!expanded.value) {
      const rows = await api<Execution[]>(
        `/journeys/${props.journeyId}/executions?${executionParams()}`,
      );
      if (
        expected !==
        JSON.stringify([
          customer.value,
          filter.value,
          version.value,
          days.value,
          selectedNode.value,
          branch.value,
          exitFilter.value,
          nodeCurrent.value,
        ])
      ) {
        refreshQueued = true;
        return;
      }
      executions.value = rows;
      hasMore.value = rows.length === pageSize;
    }
    const id = selected.value?.id;
    const request = selectionRequest;
    if (id) {
      const detail = await api<Execution>('/executions/' + id);
      if (!disposed && request === selectionRequest && selected.value?.id === id)
        selected.value = detail;
    }
    stamp.value = new Date().toLocaleTimeString();
    if (nodeTab.value === 'messages' && !messageMore.value) void loadMessages();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
    if (!disposed && !inactive && refreshQueued) {
      refreshQueued = false;
      void refresh();
    } else if (!disposed && !inactive) {
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (poll.value && !document.hidden) void refresh();
        else schedule();
      }, 5000);
    }
  }
}
function schedule() {
  if (disposed || inactive) return;
  clearTimeout(timer);
  timer = setTimeout(() => {
    if (poll.value && !document.hidden) void refresh();
    else schedule();
  }, 5000);
}
async function track(id: string, fromRoute = false) {
  if (!fromRoute) routeRestore++;
  const request = ++selectionRequest;
  deliveryRequest++;
  tracking.value = true;
  selected.value = undefined;
  delivery.value = undefined;
  try {
    const detail = await api<Execution>('/executions/' + id);
    if (detail.journey_id !== props.journeyId)
      throw new Error('This execution belongs to a different journey.');
    if (disposed || request !== selectionRequest) return;
    selected.value = detail;
    if (!fromRoute) emit('selection', { executionID: id, deliveryID: undefined });
    const v = versions.value.find((v) => v.id === selected.value?.version_id);
    if (v && v.version !== version.value) {
      version.value = v.version;
      insights.value = undefined;
      selectedNode.value = '';
      await graph();
    }
    await nextTick();
    canvas.value?.focus(detail.current_node_id);
    await refresh();
  } catch (cause) {
    if (request === selectionRequest) error.value = errorMessage(cause);
  } finally {
    if (request === selectionRequest) tracking.value = false;
  }
}
async function control(action: string) {
  if (!selected.value) return;
  try {
    await api(`/executions/${selected.value.id}/${action}`, 'POST');
    await refresh();
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function inspect(id: string, fromRoute = false) {
  const request = ++deliveryRequest;
  const execution = selected.value?.id;
  try {
    const result = await api<typeof delivery.value>('/deliveries/' + id);
    if (result?.execution_id !== execution)
      throw new Error('This message belongs to a different execution.');
    if (!disposed && request === deliveryRequest && execution === selected.value?.id) {
      delivery.value = result;
      if (!fromRoute) emit('selection', { executionID: execution, deliveryID: id });
    }
  } catch (cause) {
    if (request === deliveryRequest) error.value = errorMessage(cause);
  }
}
let activatedBefore = false;
async function restoreSelection() {
  const current = ++routeRestore;
  const id = props.initialExecutionId;
  if (!id) {
    selectionRequest++;
    deliveryRequest++;
    selected.value = undefined;
    delivery.value = undefined;
    return;
  }
  if (selected.value?.id !== id) {
    if (!versions.value.length) await refresh();
    if (current !== routeRestore) return;
    await track(id, true);
  }
  if (current !== routeRestore) return;
  if (props.initialDeliveryId && selected.value && delivery.value?.id !== props.initialDeliveryId)
    await inspect(props.initialDeliveryId, true);
  if (!props.initialDeliveryId) delivery.value = undefined;
}
watch(
  () => [props.initialExecutionId, props.initialDeliveryId],
  () => void restoreSelection(),
);
onMounted(async () => {
  await refresh();
  if (props.initialExecutionId) await restoreSelection();
});
onActivated(() => {
  inactive = false;
  if (activatedBefore) void refresh();
  activatedBefore = true;
});
onDeactivated(() => {
  inactive = true;
  clearTimeout(timer);
});
onBeforeUnmount(() => {
  disposed = true;
  clearTimeout(timer);
});
</script>
<template>
  <section
    class="journey-observe"
    :aria-busy="busy"
    :class="{ 'analytics-mode': mode === 'analytics' }"
  >
    <div class="page-heading">
      <div>
        <h2>{{ mode === 'analytics' ? 'Journey analytics' : 'Live Monitor' }}</h2>
        <p>
          {{
            mode === 'analytics'
              ? 'Follow the outcome back to its people, steps and messages.'
              : 'See where customers are, what they are waiting for and what happens next.'
          }}
        </p>
      </div>
      <button :disabled="busy" @click="refresh(true)">
        <AppIcon name="refresh" :size="15" />Refresh
        {{ mode === 'analytics' ? 'metrics' : 'monitor' }}
      </button>
    </div>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <div class="observe-toolbar">
      <ScopeSelect
        :model-value="String(version)"
        label="Published version"
        :options="
          versions.map((v) => ({ value: String(v.version), label: 'Version ' + v.version }))
        "
        @update:model-value="
          version = Number($event);
          changeScope();
        "
      /><ScopeSelect
        :model-value="String(days)"
        label="Entry period"
        :options="[
          { value: '0', label: 'All time' },
          { value: '7', label: 'Last 7 days' },
          { value: '30', label: 'Last 30 days' },
          { value: '90', label: 'Last 90 days' },
        ]"
        @update:model-value="
          days = Number($event);
          changePeriod();
        "
      /><ScopeSelect
        v-if="workflow"
        :model-value="selectedNode || 'all'"
        label="Step"
        :options="[
          { value: 'all', label: 'All steps' },
          ...workflow.nodes.map((n) => ({ value: n.id, label: n.label })),
        ]"
        @update:model-value="
          (id) => {
            if (id === 'all') browse();
            else {
              // Choosing a step shows it on the map.
              section = 'map';
              selectNode(id);
              nextTick(() => canvas?.focus(id));
            }
          }
        "
      />
      <span class="observe-freshness" role="status"
        >{{ freshness
        }}<small
          >Counters
          {{
            insights?.updated_at ? new Date(insights.updated_at).toLocaleTimeString() : 'pending'
          }}
          · refreshed {{ stamp || '—' }} · UTC entry cohorts</small
        ></span
      ><label v-if="mode !== 'analytics'" class="checkbox-label"
        ><input v-model="poll" type="checkbox" />Auto refresh · 5s</label
      >
    </div>
    <AudienceEntry
      v-if="mode !== 'analytics'"
      :journey-id="journeyId"
      :can-run="admin && !readonly"
      @ran="refresh(true)"
    />
    <p v-if="!versions.length" class="panel">
      Publish a validated journey to monitor live executions.
    </p>
    <div v-if="busy && !insights" class="panel">
      <SkeletonBlock label="Loading journey breakdowns…" variant="cards" :rows="4" />
    </div>
    <template v-if="workflow && insights">
      <p v-if="insights.overdue" class="warning-message">
        <button @click="browseOverdue">Inspect {{ insights.overdue }} waits past due</button> ·
        Journeys on pause are excluded.
      </p>
      <p v-if="!insights.coverage_complete" class="warning-message">
        Historical breakdowns are being prepared. Counts are provisional until the backfill
        completes.
      </p>
      <div class="observe-kpis">
        <MetricCard
          label="Journey entries"
          :value="value('execution', 'entries')"
          :caption="
            insights.unique_customers === null
              ? 'Logical enrollments'
              : insights.unique_customers + ' unique customers'
          "
          icon="users"
          @select="browse()"
        /><MetricCard
          label="Goal reached"
          :value="value('execution', 'converted')"
          :caption="
            (value('execution', 'entries')
              ? ((100 * value('execution', 'converted')) / value('execution', 'entries')).toFixed(1)
              : '0') + '% of selected entries'
          "
          icon="chart"
          tone="green"
          @select="browse('', 'converted')"
        /><MetricCard
          label="In progress"
          :value="value('execution', 'RUNNING') + value('execution', 'WAITING')"
          :caption="value('execution', 'WAITING') + ' waiting now in this cohort'"
          icon="clock"
          @select="browse('active')"
        /><MetricCard
          label="Needs attention"
          :value="value('execution', 'FAILED')"
          :caption="
            insights.journey_status === 'Paused'
              ? 'Journey paused · timers held'
              : insights.overdue + ' waits past due by over 30s'
          "
          icon="events"
          tone="amber"
          @select="browse('FAILED')"
        />
      </div>
      <section v-if="mode === 'analytics'" class="surface observe-trend">
        <div class="surface-heading">
          <div>
            <h3>Entries &amp; goals over time</h3>
            <p>Grouped by entry day; later outcomes update that cohort.</p>
          </div>
          <span class="pill">{{ days ? days + ' days' : 'All time' }} · UTC</span>
        </div>
        <TrendChart :data="chartData" />
      </section>
      <nav v-if="sectioned" class="monitor-sections" aria-label="Monitor sections">
        <button
          v-for="part in [
            { id: 'map', label: 'Journey map', icon: 'workflow' },
            { id: 'people', label: 'Executions', icon: 'users' },
            { id: 'messages', label: 'Messages', icon: 'email' },
          ] as const"
          :key="part.id"
          type="button"
          :aria-current="section === part.id ? 'page' : undefined"
          @click="openSection(part.id)"
        >
          <AppIcon :name="part.icon" :size="15" />{{ part.label }}
        </button>
      </nav>
      <div
        class="observe-layout"
        :class="{
          'has-details': selectedNode || selected || showMessages,
          'messages-only': sectioned && section === 'messages',
          'people-only': sectioned && section === 'people',
        }"
      >
        <div v-if="!sectioned || section !== 'messages'" class="observe-main">
          <JourneyMetricsCanvas
            v-if="shows('map')"
            ref="canvas"
            :workflow="workflow"
            :insights="insights"
            :selected-node="selectedNode || selected?.current_node_id"
            :execution="selected"
            @select="selectNode"
          />
          <section v-if="shows('people')" ref="people" class="surface observe-people">
            <div class="surface-heading">
              <div>
                <h3>Executions{{ selectedStep ? ' · ' + selectedStep.label : '' }}</h3>
                <p>
                  {{ breakdown.length ? breakdown.join(' · ') + ' · ' : '' }}Published v{{
                    version
                  }}
                  · {{ days ? 'selected entry period' : 'all entries' }}
                </p>
              </div>
              <button v-if="breakdown.length" @click="browse()">Clear breakdown</button>
            </div>
            <div class="observe-record-filters">
              <label
                >Track customer ID<input
                  v-model="customer"
                  placeholder="Find an enrolled customer…"
                  @keyup.enter="refresh(true)" /></label
              ><button @click="refresh(true)">Track customer</button
              ><ScopeSelect
                v-model="filter"
                label="Execution state"
                :options="[
                  { value: 'all', label: 'All states' },
                  { value: 'active', label: 'In progress' },
                  ...['RUNNING', 'WAITING', 'COMPLETED', 'EXITED', 'FAILED', 'CANCELLED'].map(
                    (s) => ({ value: s, label: s.replaceAll('_', ' ') }),
                  ),
                ]"
                @update:model-value="
                  filter = $event === 'all' ? '' : $event;
                  refresh(true);
                "
              />
            </div>
            <p v-if="expanded" class="table-caption">
              Browsing earlier entries. Refresh monitor to return to the latest.
            </p>
            <div class="observe-record-list">
              <p v-if="!executions.length" class="empty-inline">
                No matching executions in this scope.
              </p>
              <button
                v-for="e in executions"
                :key="e.id"
                class="list-row"
                :class="{ selected: selected?.id === e.id }"
                @click="track(e.id)"
              >
                <span
                  >{{ e.customer_id }} · {{ e.state
                  }}<small
                    >{{ stepLabel(e.current_node_id)
                    }}{{ e.exit_reason ? ' · ' + reasonLabel(e.exit_reason) : '' }}</small
                  ></span
                ><time>{{ time(e.started_at) }}</time>
              </button>
            </div>
            <button v-if="hasMore" :disabled="paging" class="load-more" @click="loadMore">
              {{ paging ? 'Loading…' : 'Load more executions' }}
            </button>
          </section>
        </div>
        <aside
          v-if="selectedNode || selected || tracking || showMessages"
          class="observe-details"
          aria-label="Journey breakdown"
        >
          <SkeletonBlock v-if="tracking" label="Loading execution…" variant="text" :rows="4" />
          <section v-if="selected" class="execution-inspector" aria-label="Execution details">
            <div class="surface-heading">
              <h3>Customer journey</h3>
              <button aria-label="Close customer journey" @click="clearSelection">Close</button>
            </div>
            <div class="toolbar">
              <button @click="emit('inspect', { kind: 'execution', id: selected.id })">
                Open activity inspector</button
              ><button
                v-if="selected.entry_event_id"
                @click="emit('inspect', { kind: 'event', id: selected.entry_event_id })"
              >
                View entry event
              </button>
            </div>
            <button
              v-if="!readonly && ['RUNNING', 'WAITING'].includes(selected.state)"
              @click="control('cancel')"
            >
              Cancel execution</button
            ><button v-if="admin && selected.state === 'FAILED'" @click="control('retry')">
              Recover failed job
            </button>
            <div class="execution-overview">
              <StatusBadge :status="selected.state" /><span>Version {{ version }}</span
              ><span>{{ selected.customer_id }}</span
              ><span v-if="selected.exit_reason"
                >Exit: {{ selected.exit_reason.replaceAll('_', ' ') }}</span
              >
            </div>
            <dl class="execution-facts">
              <div>
                <dt>Entered</dt>
                <dd>{{ time(selected.started_at) }}</dd>
              </div>
              <div>
                <dt>Current step</dt>
                <dd>{{ stepLabel(selected.current_node_id) }}</dd>
              </div>
              <div>
                <dt>Execution / correlation ID</dt>
                <dd>{{ selected.id }}</dd>
              </div>
            </dl>
            <h4>Transitions &amp; conditions</h4>
            <div class="execution-timeline">
              <div
                v-for="step in selected.steps"
                :key="step.node_id"
                class="timeline-step"
                :class="step.state.toLowerCase()"
              >
                <span class="timeline-node"
                  ><AppIcon
                    :name="
                      step.state === 'WAITING'
                        ? 'clock'
                        : step.state === 'FAILED'
                          ? 'close'
                          : 'check'
                    "
                    :size="15"
                /></span>
                <div class="timeline-step-content">
                  <div>
                    <strong>{{ stepLabel(step.node_id) }}</strong
                    ><StatusBadge :status="step.state" />
                  </div>
                  <p>{{ decisionText(step.output) }}</p>
                  <time
                    >{{ time(step.started_at)
                    }}<span v-if="step.completed_at">
                      · finished {{ time(step.completed_at) }}</span
                    ></time
                  >
                </div>
              </div>
            </div>
            <h4>Wait deadlines &amp; scheduled jobs</h4>
            <p
              v-if="
                !selected.waiters.some((w) => w.state === 'WAITING') &&
                !selected.jobs.some((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))
              "
              class="muted"
            >
              No pending waits or scheduled work.
            </p>
            <div
              v-for="waiter in selected.waiters.filter((w) => w.state === 'WAITING')"
              :key="waiter.node_id"
              class="execution-wait"
            >
              <AppIcon name="events" :size="17" />
              <div>
                <strong>Waiting for {{ waiter.event_name }}</strong>
                <p>{{ stepLabel(waiter.node_id) }} · until {{ time(waiter.deadline) }}</p>
              </div>
            </div>
            <div
              v-for="job in selected.jobs.filter((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))"
              :key="job.id"
              class="execution-wait"
            >
              <AppIcon name="clock" :size="17" />
              <div>
                <strong>{{ stepLabel(job.node_id) }}</strong>
                <p>{{ job.kind }} · due {{ time(job.due_at) }} · {{ job.attempts }} attempts</p>
              </div>
              <StatusBadge :status="job.state" />
            </div>
            <h4>Messages &amp; decisions</h4>
            <p v-if="!selected.deliveries.length" class="muted">
              No logical messages created for this execution.
            </p>
            <button
              v-for="d in selected.deliveries"
              :key="d.id"
              class="list-row"
              @click="inspect(d.id)"
            >
              <span
                >{{ stepLabel(d.node_id) }} · {{ d.channel.toUpperCase() }} · {{ d.status }} ·
                {{ d.attempts }} attempts</span
              ><AppIcon name="arrow" :size="15" />
            </button>
            <section v-if="delivery" class="delivery-inspector">
              <div class="surface-heading">
                <h4>Delivery decision &amp; attempts</h4>
                <button @click="inspect(delivery.id)">Refresh delivery</button>
              </div>
              <DecisionSummary :decision="delivery.decision" /><AttemptHistory
                :attempts="delivery.attempt_history"
                :provider="delivery.provider"
                :response="delivery.last_error"
                :engagement="delivery.engagement"
              />
            </section>
            <details class="execution-raw">
              <summary>Advanced execution payload</summary>
              <pre>{{ JSON.stringify(selected, null, 2) }}</pre>
            </details>
          </section>
          <section
            v-if="(selectedStep || showMessages) && !selected"
            :aria-label="selectedStep ? 'Step breakdown' : 'Message breakdown'"
          >
            <div class="surface-heading">
              <div>
                <small>{{ selectedStep ? 'STEP DETAILS' : 'MESSAGE DETAILS' }}</small>
                <h3>{{ selectedStep?.label || 'Journey messages' }}</h3>
              </div>
              <button
                v-if="!sectioned || section !== 'messages'"
                aria-label="Close step breakdown"
                @click="
                  selectedNode = '';
                  showMessages = false;
                  if (sectioned && section === 'messages') section = 'map';
                  refresh(true);
                "
              >
                Close
              </button>
            </div>
            <TabsRoot v-model="nodeTab"
              ><TabsList class="observe-tabs" aria-label="Step detail tabs"
                ><TabsTrigger v-if="selectedStep" value="summary">Summary</TabsTrigger
                ><TabsTrigger v-if="selectedStep" value="people">People</TabsTrigger
                ><TabsTrigger
                  v-if="
                    showMessages || selectedStep?.type === 'email' || selectedStep?.type === 'sms'
                  "
                  value="messages"
                  >Messages</TabsTrigger
                ></TabsList
              ><TabsContent value="summary"
                ><dl class="node-detail-stats">
                  <div>
                    <dt>Reached step</dt>
                    <dd>{{ nodeValue('entered') }}</dd>
                  </div>
                  <div>
                    <dt>Waiting now</dt>
                    <dd>{{ nodeValue('WAITING', 'node_current') }}</dd>
                  </div>
                  <div>
                    <dt>Step completed</dt>
                    <dd>{{ nodeValue('SUCCEEDED') }}</dd>
                  </div>
                  <div>
                    <dt>
                      {{
                        selectedStep?.type === 'email' || selectedStep?.type === 'sms'
                          ? 'Provider accepted'
                          : 'Processing now'
                      }}
                    </dt>
                    <dd>
                      {{
                        selectedStep?.type === 'email' || selectedStep?.type === 'sms'
                          ? nodeValue('ACCEPTED', 'node_delivery')
                          : nodeValue('RUNNING', 'node_current')
                      }}
                    </dd>
                  </div>
                </dl>
                <dl
                  v-if="selectedStep?.type === 'email' || selectedStep?.type === 'sms'"
                  class="node-detail-stats"
                  aria-label="Engagement"
                >
                  <div v-if="selectedStep?.type === 'email'">
                    <dt>Opened</dt>
                    <dd>{{ nodeValue('opened', 'node_delivery') }}</dd>
                  </div>
                  <div>
                    <dt>Clicked a link</dt>
                    <dd>{{ nodeValue('clicked', 'node_delivery') }}</dd>
                  </div>
                  <div>
                    <dt>Bounced</dt>
                    <dd>{{ nodeValue('bounced', 'node_delivery') }}</dd>
                  </div>
                  <div v-if="selectedStep?.type === 'email'">
                    <dt>Unsubscribed</dt>
                    <dd>{{ nodeValue('unsubscribed', 'node_delivery') }}</dd>
                  </div>
                </dl>
                <p
                  v-if="selectedStep?.type === 'email' || selectedStep?.type === 'sms'"
                  class="help-text"
                >
                  Completing a message step schedules the message; provider acceptance is a separate
                  result. Each person counts once however often they open or click. Opens count when
                  the mail client loads images, which some clients do on their own.
                </p>
                <h4>Paths taken</h4>
                <button
                  v-for="edge in workflow.edges.filter((e) => e.source === selectedNode)"
                  :key="edge.id"
                  class="list-row"
                  @click="branchPeople(edge.port)"
                >
                  <span>{{ portLabel(edge.port) }} → {{ stepLabel(edge.target) }}</span
                  ><strong>{{ value('edge', 'traversed', selectedNode + ':' + edge.port) }}</strong>
                </button>
                <h4>Message decisions</h4>
                <p v-if="!nodeReasons.length" class="help-text">
                  No blocked or failed decisions in this scope.
                </p>
                <button
                  v-for="r in nodeReasons"
                  :key="r.metric"
                  class="list-row"
                  @click="reasonMessages(r.metric)"
                >
                  <span>{{ reasonLabel(r.metric) }}</span
                  ><strong>{{ r.value }}</strong>
                </button>
                <div
                  v-if="
                    insights.metrics.some(
                      (m) => m.dimension === 'node_exit' && m.key === selectedNode && m.value > 0,
                    )
                  "
                >
                  <h4>Global exits from this step</h4>
                  <button
                    v-for="r in insights.metrics.filter(
                      (m) => m.dimension === 'node_exit' && m.key === selectedNode && m.value > 0,
                    )"
                    :key="r.metric"
                    class="list-row"
                    @click="
                      exitFilter = r.metric;
                      nodeCurrent = true;
                      branch = '';
                      filter = 'EXITED';
                      nodeTab = 'people';
                      refresh(true);
                    "
                  >
                    <span>{{ reasonLabel(r.metric) }}</span
                    ><strong>{{ r.value }}</strong>
                  </button>
                  <p class="help-text">
                    These customers left directly from this step, without visiting an Exit node.
                  </p>
                </div></TabsContent
              >
              <TabsContent value="people"
                ><p>
                  The execution list shows the people behind this step{{
                    branch ? ' and selected branch' : ''
                  }}. Select a person to inspect their path and timeline.
                </p>
                <button
                  @click="
                    branch = '';
                    filter = 'WAITING';
                    nodeCurrent = true;
                    refresh(true);
                  "
                >
                  Show people waiting here
                </button></TabsContent
              >
              <TabsContent value="messages"
                ><div class="observe-record-filters">
                  <ScopeSelect
                    v-model="messageChannel"
                    label="Channel"
                    :options="[
                      { value: 'all', label: 'All channels' },
                      { value: 'email', label: 'Email' },
                      { value: 'sms', label: 'SMS' },
                    ]"
                    @update:model-value="
                      messageChannel = $event === 'all' ? '' : $event;
                      loadMessages();
                    "
                  /><ScopeSelect
                    v-model="messageStatus"
                    label="Message status"
                    :options="[
                      { value: 'all', label: 'All outcomes' },
                      ...[
                        'ACCEPTED',
                        'BLOCKED',
                        'FAILED',
                        'UNKNOWN',
                        'SKIPPED',
                        'CANCELLED',
                        'SCHEDULED',
                        'CLAIMED',
                        'DISPATCHING',
                      ].map((s) => ({ value: s, label: s })),
                    ]"
                    @update:model-value="
                      messageStatus = $event === 'all' ? '' : $event;
                      loadMessages();
                    "
                  />
                </div>
                <p v-if="messageReason" class="help-text">
                  Reason: {{ reasonLabel(messageReason) }}
                  <button
                    @click="
                      messageReason = '';
                      loadMessages();
                    "
                  >
                    Clear reason
                  </button>
                </p>
                <SkeletonBlock v-if="messageBusy" label="Loading messages…" variant="text" />
                <p v-if="!messages.length && !messageBusy" class="empty-inline">
                  No matching messages.
                </p>
                <button
                  v-for="m in messages"
                  :key="m.id"
                  class="list-row"
                  @click="
                    track(m.execution_id).then(() => {
                      if (selected?.id === m.execution_id) return inspect(m.id);
                    })
                  "
                >
                  <span
                    >{{ m.customer_id
                    }}<small
                      >{{ m.channel.toUpperCase() }} · {{ m.status }} ·
                      {{ m.attempts }} attempts</small
                    ></span
                  ></button
                ><button v-if="messageMore" :disabled="messageBusy" @click="loadMessages(true)">
                  Load more messages
                </button></TabsContent
              ></TabsRoot
            >
          </section>
        </aside>
      </div>
      <PathFlow
        v-if="mode === 'analytics' && workflow && insights && version"
        :workflow="workflow"
        :insights="insights"
        @branch="variantPeople($event.node, $event.port)"
        @step="stepPeople($event)"
      />
      <ExperimentResults
        v-if="mode === 'analytics' && workflow"
        :journey-id="journeyId"
        :version="version"
        :workflow="workflow"
        @people="variantPeople($event.node, $event.port)"
      />
      <div v-if="mode === 'analytics'" class="observe-analytics-bottom">
        <section class="surface">
          <div class="surface-heading">
            <h3>Step reach · workflow order</h3>
            <p>Alternative branches are not sequential drop-off.</p>
          </div>
          <div class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Step</th>
                  <th>Reached</th>
                  <th>Completed step</th>
                  <th>Waiting now</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="n in graphOrder" :key="n.id">
                  <td>
                    <button
                      class="journey-name-button"
                      @click="
                        selectNode(n.id);
                        canvas?.focus(n.id);
                      "
                    >
                      {{ n.label }}
                    </button>
                  </td>
                  <td>
                    <button
                      v-if="value('node', 'entered', n.id)"
                      class="metric-link"
                      :aria-label="`Show ${value('node', 'entered', n.id)} executions that reached ${n.label}`"
                      @click="stepPeople(n.id)"
                    >
                      {{ value('node', 'entered', n.id) }}</button
                    ><template v-else>0</template>
                  </td>
                  <td>{{ value('node', 'SUCCEEDED', n.id) }}</td>
                  <td>
                    <button
                      v-if="value('node_current', 'WAITING', n.id)"
                      class="metric-link"
                      :aria-label="`Show ${value('node_current', 'WAITING', n.id)} executions waiting at ${n.label}`"
                      @click="stepPeople(n.id, true)"
                    >
                      {{ value('node_current', 'WAITING', n.id) }}</button
                    ><template v-else>0</template>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="surface">
          <div class="surface-heading"><h3>Journey outcomes</h3></div>
          <button v-for="r in exitReasons" :key="r.key" class="list-row" @click="browse('', r.key)">
            <span>{{ reasonLabel(r.key) }}</span
            ><strong>{{ r.value }}</strong>
          </button>
          <p class="table-caption">
            {{ value('execution', 'finished_count') }} finished runs · mean terminal duration
            {{
              (
                value('execution', 'duration_ms') /
                Math.max(1, value('execution', 'finished_count')) /
                1000
              ).toFixed(1)
            }}
            seconds
          </p>
        </section>
      </div>
      <section v-if="mode === 'analytics'" class="surface">
        <div class="surface-heading">
          <div>
            <h3>Channel performance</h3>
            <p>Selected entry cohort · retries stay under one logical message.</p>
          </div>
        </div>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Channel</th>
                <th>Planned</th>
                <th>Provider accepted</th>
                <th>Blocked</th>
                <th>Failed</th>
                <th>Unknown</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in ['email', 'sms']" :key="c">
                <td>{{ c.toUpperCase() }}</td>
                <td>
                  <button class="metric-link" @click="browseChannel(c, '')">
                    {{ value('channel', 'scheduled', c) }}
                  </button>
                </td>
                <td>
                  <button class="metric-link" @click="browseChannel(c, 'ACCEPTED')">
                    {{ value('channel', 'ACCEPTED', c) }}
                  </button>
                </td>
                <td>
                  <button class="metric-link" @click="browseChannel(c, 'BLOCKED')">
                    {{ value('channel', 'BLOCKED', c) }}
                  </button>
                </td>
                <td>
                  <button class="metric-link" @click="browseChannel(c, 'FAILED')">
                    {{ value('channel', 'FAILED', c) }}
                  </button>
                </td>
                <td>
                  <button class="metric-link" @click="browseChannel(c, 'UNKNOWN')">
                    {{ value('channel', 'UNKNOWN', c) }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="table-caption">
          Provider accepted is not confirmed delivery. Channel conversion attribution and open/click
          tracking are not available.
        </p>
      </section>
    </template>
  </section>
</template>
