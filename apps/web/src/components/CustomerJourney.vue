<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import JourneyMetricsCanvas from './JourneyMetricsCanvas.vue';
import JourneyRoute from './JourneyRoute.vue';
import AppIcon from './ui/AppIcon.vue';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import StatusBadge from './ui/StatusBadge.vue';
import { api } from '../lib/api';
import { errorMessage, formatDateTime } from '../lib/format';
import { journeyPath, type ExecutionDetail } from '../lib/customer-journey';
import type { InspectionKind } from '../lib/navigation';
import { loadDefinition } from '../lib/journey-definition';
import type { Workflow } from '../lib/workflow';
import { stepSummary } from '../lib/workflow-presentation';

// One person's route through a journey: the map shows where they went, the route says what
// happened at each stop.
export type CustomerExecution = {
  id: string;
  journey_id: string;
  version_id: string;
  state: string;
  exit_reason?: string | null;
  started_at: string;
};
const props = defineProps<{
  executions: CustomerExecution[];
  journeyNames: Record<string, string>;
}>();
const emit = defineEmits<{
  inspect: [{ kind: InspectionKind; id: string }];
  monitor: [{ journeyID: string; executionID: string }];
}>();
const selectedID = ref('');
const detail = ref<ExecutionDetail>();
const workflow = ref<Workflow>();
const version = ref(0);
const busy = ref(false);
const error = ref('');
const focused = ref('');
const canvas = ref<InstanceType<typeof JourneyMetricsCanvas>>();
let sequence = 0;

const journeyName = (id: string) => props.journeyNames[id] || 'Journey';
const selected = computed(() => props.executions.find((item) => item.id === selectedID.value));
const path = computed(() =>
  workflow.value && detail.value
    ? journeyPath(workflow.value, detail.value, stepSummary)
    : undefined,
);

async function load() {
  const execution = selected.value;
  const current = ++sequence;
  detail.value = undefined;
  focused.value = '';
  error.value = '';
  if (!execution) return;
  busy.value = true;
  try {
    const [run, graph] = await Promise.all([
      api<ExecutionDetail>('/executions/' + execution.id),
      loadDefinition(execution.journey_id, execution.version_id),
    ]);
    if (current !== sequence) return;
    workflow.value = graph.workflow;
    version.value = graph.version;
    detail.value = run;
  } catch (cause) {
    if (current === sequence) error.value = errorMessage(cause);
  } finally {
    if (current === sequence) busy.value = false;
  }
}
function focus(nodeID: string) {
  focused.value = nodeID;
  canvas.value?.focus(nodeID);
}
watch(
  () => props.executions,
  (list) => {
    if (!list.some((item) => item.id === selectedID.value)) selectedID.value = list[0]?.id || '';
  },
  { immediate: true },
);
watch(selectedID, () => void load(), { immediate: true });
onBeforeUnmount(() => sequence++);
</script>
<template>
  <section class="customer-journey" aria-label="Customer journey">
    <div v-if="executions.length > 1" class="run-picker" role="group" aria-label="Journey entries">
      <button
        v-for="execution in executions"
        :key="execution.id"
        type="button"
        class="run-option"
        :aria-pressed="execution.id === selectedID"
        @click="selectedID = execution.id"
      >
        <span class="run-option-name">{{ journeyName(execution.journey_id) }}</span>
        <span class="run-option-meta"
          ><StatusBadge :status="execution.state" />{{ formatDateTime(execution.started_at) }}</span
        >
      </button>
    </div>
    <p v-if="error" role="alert" class="inline-alert error-alert">
      {{ error }}<button type="button" @click="load">Try again</button>
    </p>
    <SkeletonBlock v-if="busy && !detail" label="Loading journey…" variant="cards" :rows="2" />
    <template v-if="selected && detail && workflow && path">
      <header class="run-summary">
        <div class="run-title">
          <span class="run-mark"><AppIcon name="workflow" :size="18" /></span>
          <div>
            <h3>{{ journeyName(detail.journey_id) }}</h3>
            <p>
              <strong>{{ path.headline }}</strong> · Version {{ version }} · entered
              {{ formatDateTime(detail.started_at)
              }}<template v-if="path.duration"> · took {{ path.duration }}</template>
            </p>
          </div>
        </div>
        <div class="run-actions">
          <StatusBadge :status="detail.state" />
          <button type="button" @click="emit('inspect', { kind: 'execution', id: detail.id })">
            Inspect details
          </button>
          <button
            type="button"
            @click="emit('monitor', { journeyID: detail.journey_id, executionID: detail.id })"
          >
            Open in Live Monitor
          </button>
        </div>
      </header>
      <div class="journey-body">
        <JourneyMetricsCanvas
          ref="canvas"
          :workflow="workflow"
          :execution="detail"
          personal
          :selected-node="focused || detail.current_node_id"
          @select="focused = $event"
        />
        <JourneyRoute
          class="route-panel"
          :path="path"
          :focused="focused"
          focusable
          @focus="focus"
          @message="emit('inspect', { kind: 'message', id: $event })"
        />
      </div>
    </template>
  </section>
</template>
<style scoped>
.customer-journey {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.run-picker {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}
.run-option {
  flex: 0 0 auto;
  display: grid;
  gap: 6px;
  justify-items: start;
  min-width: 190px;
  padding: 10px 12px;
  text-align: left;
  border-radius: 10px;
}
.run-option[aria-pressed='true'] {
  border-color: var(--accent);
  background: var(--accent-soft);
}
.run-option-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}
.run-option-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--muted);
}
.run-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.run-title {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.run-mark {
  display: grid;
  place-items: center;
  flex: 0 0 36px;
  height: 36px;
  border-radius: 9px;
  background: var(--accent-soft);
  color: var(--accent-text);
}
.run-title h3 {
  margin: 0;
  font-size: 15px;
}
.run-title p {
  margin: 3px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}
.run-title strong {
  color: var(--text);
  font-weight: 600;
}
.run-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.journey-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
  gap: 16px;
  align-items: start;
}
.journey-body :deep(.metrics-canvas) {
  height: 600px;
}
.route-panel {
  max-height: 600px;
  overflow: auto;
  padding: 14px 14px 6px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
@media (max-width: 1180px) {
  .journey-body {
    grid-template-columns: minmax(0, 1fr);
  }
  .journey-body :deep(.metrics-canvas) {
    height: 420px;
  }
  .route-panel {
    max-height: none;
  }
}
</style>
