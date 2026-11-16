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
import JourneyMetricsCanvas from './JourneyMetricsCanvas.vue';
import ExperimentResults from './ExperimentResults.vue';
import AudienceEntry from './AudienceEntry.vue';
import PathFlow from './PathFlow.vue';
import ScopeSelect from './ui/ScopeSelect.vue';
import TrendChart from './ui/TrendChart.vue';
import { portLabel } from '../lib/workflow-presentation';
import { reasonLabel, type Insights } from '../lib/insights';
import { api } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import MonitorKpis from './MonitorKpis.vue';
import ExecutionDetails from './ExecutionDetails.vue';
import StepReachTable from './StepReachTable.vue';
import JourneyOutcomes from './JourneyOutcomes.vue';
import ChannelPerformance from './ChannelPerformance.vue';
import StepBreakdown, { type JourneyMessage } from './StepBreakdown.vue';
import { utcTime, type DeliveryDetail, type Execution } from '../lib/executions';
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
const messages = ref<JourneyMessage[]>([]);
const messageStatus = ref(''),
  messageChannel = ref(''),
  messageReason = ref('');
const messageMore = ref(false),
  messageBusy = ref(false);
let messageRequest = 0;
const selectedStep = computed(() => workflow.value?.nodes.find((n) => n.id === selectedNode.value));
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
const freshness = computed(() => {
  const d = insights.value;
  if (!d) return 'Loading metrics…';
  if (!d.coverage_complete) return 'Preparing historical breakdowns';
  if (!d.projector_last_success || Date.now() - Date.parse(d.projector_last_success) > 15000)
    return 'Counters may be delayed';
  if (d.queue_oldest_at && Date.now() - Date.parse(d.queue_oldest_at) > 10000)
    return 'Counters catching up';
  return 'Counters up to date';
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
// On analytics a step count only filters the executions list; the step drawer opens when the
// step itself is chosen on the map, so it never covers the list it filters.
const stepDetails = ref(false);
const detailsOpen = computed(
  () =>
    !!(selected.value || tracking.value || showMessages.value) ||
    (!!selectedNode.value && (props.mode !== 'analytics' || stepDetails.value)),
);
function selectNode(id: string) {
  stepDetails.value = true;
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
  stepDetails.value = false;
  showMessages.value = false;
  selectedNode.value = node;
  branchPeople(port);
}
// From a step count to the people behind it: everyone who reached the step, or
// only those waiting there now.
function stepPeople(id: string, waiting = false) {
  stepDetails.value = false;
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
function closeBreakdown() {
  selectedNode.value = '';
  showMessages.value = false;
  if (sectioned.value && section.value === 'messages') section.value = 'map';
  void refresh(true);
}
// From a step's global exits to the customers who left the journey there.
function exitPeople(reason: string) {
  exitFilter.value = reason;
  nodeCurrent.value = true;
  branch.value = '';
  filter.value = 'EXITED';
  nodeTab.value = 'people';
  void refresh(true);
}
function waitingHere() {
  branch.value = '';
  filter.value = 'WAITING';
  nodeCurrent.value = true;
  void refresh(true);
}
// A message opens its execution, then its delivery inside it.
function openMessage(message: JourneyMessage) {
  void track(message.execution_id).then(() => {
    if (selected.value?.id === message.execution_id) return inspect(message.id);
  });
}
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
const delivery = ref<DeliveryDetail>();
const stepLabel = (id: string) => workflow.value?.nodes.find((node) => node.id === id)?.label || id;
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
    const result = await api<DeliveryDetail>('/deliveries/' + id);
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
// On the analytics page details open as a drawer over the page; Escape closes it.
function closeDetails(event: KeyboardEvent) {
  if (event.key !== 'Escape' || props.mode !== 'analytics' || inactive) return;
  // A dialog over the drawer (the activity inspector) takes Escape first.
  if (event.defaultPrevented || document.querySelector('dialog[open]')) return;
  if (selected.value || tracking.value) clearSelection();
  else if (selectedNode.value || showMessages.value) {
    selectedNode.value = '';
    showMessages.value = false;
    void refresh(true);
  }
}
onMounted(async () => {
  document.addEventListener('keydown', closeDetails);
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
  document.removeEventListener('keydown', closeDetails);
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
      <MonitorKpis :insights="insights" @browse="browse" />
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
          'has-details': detailsOpen,
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
          <!-- On the analytics page the list sits at the bottom, under the numbers it explains. -->
          <Teleport defer to="#analytics-executions" :disabled="mode !== 'analytics'">
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
                  ><time>{{ utcTime(e.started_at) }}</time>
                </button>
              </div>
              <button v-if="hasMore" :disabled="paging" class="load-more" @click="loadMore">
                {{ paging ? 'Loading…' : 'Load more executions' }}
              </button>
            </section>
          </Teleport>
        </div>
        <Teleport to="body" :disabled="mode !== 'analytics'">
          <aside
            v-if="detailsOpen"
            class="observe-details"
            :class="{ drawer: mode === 'analytics' }"
            aria-label="Journey breakdown"
          >
            <SkeletonBlock v-if="tracking" label="Loading execution…" variant="text" :rows="4" />
            <ExecutionDetails
              v-if="selected"
              :execution="selected"
              :delivery="delivery"
              :workflow="workflow"
              :version="version"
              :readonly="readonly"
              :admin="admin"
              @close="clearSelection"
              @inspect="emit('inspect', $event)"
              @control="control"
              @delivery="inspect"
            />
            <StepBreakdown
              v-if="(selectedStep || showMessages) && !selected && detailsOpen"
              v-model:tab="nodeTab"
              v-model:channel="messageChannel"
              v-model:status="messageStatus"
              v-model:reason="messageReason"
              :step="selectedStep"
              :node-id="selectedNode"
              :branch="branch"
              :show-messages="showMessages"
              :workflow="workflow"
              :insights="insights"
              :messages="messages"
              :busy="messageBusy"
              :more="messageMore"
              :closable="!sectioned || section !== 'messages'"
              @close="closeBreakdown"
              @branch="branchPeople"
              @reason="reasonMessages"
              @exits="exitPeople"
              @waiting="waitingHere"
              @load="loadMessages"
              @open-message="openMessage"
            />
          </aside>
        </Teleport>
      </div>
      <PathFlow
        v-if="mode === 'analytics' && workflow && insights && version"
        :workflow="workflow"
        :insights="insights"
        :journey-id="journeyId"
        :scope="scopedParams().toString()"
        @open="track($event)"
      />
      <ExperimentResults
        v-if="mode === 'analytics' && workflow"
        :journey-id="journeyId"
        :version="version"
        :workflow="workflow"
        @people="variantPeople($event.node, $event.port)"
      />
      <div v-if="mode === 'analytics'" class="observe-analytics-bottom">
        <StepReachTable
          :workflow="workflow"
          :insights="insights"
          @select="
            selectNode($event);
            canvas?.focus($event);
          "
          @people="stepPeople"
        />
        <JourneyOutcomes :insights="insights" @browse="browse('', $event)" />
      </div>
      <ChannelPerformance
        v-if="mode === 'analytics'"
        :insights="insights"
        @browse="browseChannel"
      />
      <div v-if="mode === 'analytics'" id="analytics-executions" />
    </template>
  </section>
</template>
