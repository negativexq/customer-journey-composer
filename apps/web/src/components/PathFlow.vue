<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import EmptyState from './ui/EmptyState.vue';
import { api } from '../lib/api';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { metric, reasonLabel, type Insights } from '../lib/insights';
import { pathRows, type PathRow } from '../lib/path-flow';
import type { Workflow } from '../lib/workflow';

// Where customers went, read top to bottom like the journey. Each row opens the customers
// behind its count right under it, so nothing on the page jumps.
const props = defineProps<{
  workflow: Workflow;
  insights: Insights;
  journeyId: string;
  // Version and period of the counts, as executions query parameters.
  scope: string;
}>();
const emit = defineEmits<{ open: [string] }>();

const data = computed(() =>
  pathRows(props.workflow, {
    reached: (id) => metric(props.insights, 'node', 'entered', id),
    taken: (id, port) => metric(props.insights, 'edge', 'traversed', id + ':' + port),
    waiting: (id) => metric(props.insights, 'node_current', 'WAITING', id),
  }),
);
const rows = computed(() => data.value.rows);
const summary = computed(() => data.value.summary);
const share = (value: number) =>
  summary.value.entered ? Math.round((value / summary.value.entered) * 100) : 0;
const labelOf = (id: string) => props.workflow.nodes.find((node) => node.id === id)?.label || id;
const rowName = (row: PathRow) =>
  `${row.kind === 'branch' ? labelOf(row.node) + ' → ' : ''}${row.label}${
    row.kind === 'left' || row.kind === 'waiting' ? ' at ' + labelOf(row.node) : ''
  }: ${plural(row.value, 'customer')}, ${share(row.value)}% of entries`;

type Run = {
  id: string;
  customer_id: string;
  state: string;
  current_node_id: string;
  started_at: string;
  exit_reason?: string;
};
const PAGE = 6;
const open = ref('');
const runs = ref<Run[]>([]);
const shown = ref(PAGE);
const loading = ref(false);
const failure = ref('');
let request = 0;

async function toggle(row: PathRow) {
  if (open.value === row.key) {
    open.value = '';
    return;
  }
  open.value = row.key;
  runs.value = [];
  shown.value = PAGE;
  failure.value = '';
  if (!row.value) return;
  const params = new URLSearchParams(props.scope);
  params.set('limit', '100');
  params.set('node_id', row.node);
  if (row.kind === 'branch') params.set('port', row.port);
  if (row.kind === 'waiting') {
    params.set('current', 'true');
    params.set('state', 'WAITING');
  }
  const mine = ++request;
  loading.value = true;
  try {
    let result = await api<Run[]>(`/journeys/${props.journeyId}/executions?${params}`);
    // Customers who stopped here are the ones whose run ended on this step.
    if (row.kind === 'left')
      result = result.filter((run) => run.current_node_id === row.node && run.state !== 'WAITING');
    if (mine === request) runs.value = result;
  } catch (cause) {
    if (mine === request) failure.value = errorMessage(cause);
  } finally {
    if (mine === request) loading.value = false;
  }
}
// New counts close an open list, so it never shows customers from another period.
watch(
  () => props.scope,
  () => (open.value = ''),
);
const outcome = (run: Run) =>
  run.exit_reason
    ? reasonLabel(run.exit_reason)
    : run.state === 'WAITING'
      ? 'Waiting at ' + labelOf(run.current_node_id)
      : run.state.charAt(0) + run.state.slice(1).toLowerCase();
</script>
<template>
  <section class="surface path-flow" aria-label="Customer paths">
    <div class="surface-heading">
      <div>
        <h3>Customer paths</h3>
        <p v-if="summary.entered">
          {{ plural(summary.entered, 'customer') }} entered · {{ summary.finished }} finished the
          journey<template v-if="summary.biggestLoss">
            · most left at <strong>{{ summary.biggestLoss.label }}</strong> ({{
              summary.biggestLoss.value
            }})</template
          >. Select a row to see its customers.
        </p>
      </div>
    </div>
    <EmptyState v-if="!summary.entered" icon="branch" title="No paths to draw yet" compact>
      Paths appear once customers enter this version in the selected period.
    </EmptyState>
    <ol v-else class="path-rows">
      <li
        v-for="row in rows"
        :key="row.key"
        :class="[
          'path-row',
          row.kind,
          { top: row.depth === 0, empty: !row.value, open: open === row.key },
        ]"
        :style="{ '--depth': row.depth }"
      >
        <button
          type="button"
          class="path-row-button"
          :aria-expanded="open === row.key"
          :aria-label="rowName(row)"
          @click="toggle(row)"
        >
          <span class="path-label"
            ><span v-if="row.kind === 'joins'" class="path-hint">Continues at </span
            >{{ row.label }}</span
          >
          <span class="path-count">{{ row.value }}</span>
          <span class="path-bar" aria-hidden="true"
            ><span :style="{ width: share(row.value) + '%' }"
          /></span>
          <span class="path-share">{{ share(row.value) }}%</span>
        </button>
        <div v-if="open === row.key" class="path-people">
          <p v-if="!row.value" class="path-note">Nobody took this path in the selected period.</p>
          <p v-else-if="loading" class="path-note" role="status">Loading customers…</p>
          <p v-else-if="failure" class="path-note error-message">{{ failure }}</p>
          <p v-else-if="!runs.length" class="path-note">
            These customers are outside the latest 100 runs.
          </p>
          <template v-else>
            <button
              v-for="run in runs.slice(0, shown)"
              :key="run.id"
              type="button"
              class="path-person"
              @click="emit('open', run.id)"
            >
              <strong>{{ run.customer_id }}</strong
              ><span>{{ outcome(run) }}</span
              ><time>{{ formatDateTime(run.started_at) }}</time>
            </button>
            <button
              v-if="runs.length > shown"
              type="button"
              class="path-more"
              @click="shown += PAGE"
            >
              Show {{ Math.min(PAGE, runs.length - shown) }} more · {{ shown }} of
              {{ runs.length }}
            </button>
          </template>
        </div>
      </li>
    </ol>
  </section>
</template>
<style scoped>
.path-flow {
  margin: 16px 0;
}
.surface-heading strong {
  color: var(--text);
  font-weight: 600;
}
.path-rows {
  margin: 0;
  padding: 0 0 12px;
  list-style: none;
}
.path-row {
  --indent: calc(var(--depth) * 22px);
}
.path-row.top + .path-row.top,
.path-row:not(.top) + .path-row.top {
  border-top: 1px solid var(--border);
}
.path-row-button {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) 44px minmax(80px, 1fr) 44px;
  align-items: center;
  gap: 14px;
  width: 100%;
  min-height: 44px;
  margin: 0;
  padding: 0 22px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.path-row-button:hover,
.path-row.open .path-row-button {
  background: var(--violet-97);
}
.path-row-button:focus-visible {
  outline: 2px solid var(--accent-text);
  outline-offset: -2px;
}
.path-label {
  padding-left: var(--indent);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path-row.step .path-label {
  font-weight: 500;
}
.path-row.branch .path-label {
  color: var(--accent-text-strong);
  font-weight: 600;
}
.path-row.branch .path-label::before {
  content: '↳ ';
}
.path-row.left .path-label,
.path-row.waiting .path-label,
.path-hint {
  color: var(--muted);
  font-style: italic;
}
.path-count,
.path-share {
  font-variant-numeric: tabular-nums;
  text-align: right;
}
.path-share {
  color: var(--muted);
  font-size: 12px;
}
.path-bar {
  height: 8px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--gray-96);
}
.path-bar > span {
  display: block;
  height: 100%;
  border-radius: 4px;
  background: var(--accent);
}
.path-row.branch .path-bar > span {
  background: var(--violet-72);
}
.path-row.left .path-bar > span {
  background: var(--red-70);
}
.path-row.waiting .path-bar > span {
  background: var(--amber-74);
}
.path-row.empty .path-label,
.path-row.empty .path-count {
  color: var(--muted);
}
.path-people {
  display: grid;
  gap: 2px;
  margin: 2px 22px 10px calc(40px + var(--indent));
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
}
.path-note {
  margin: 4px 6px;
  color: var(--muted);
  font-size: 13px;
}
.path-person {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr) auto;
  gap: 12px;
  align-items: center;
  min-height: 40px;
  margin: 0;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.path-person:hover {
  background: var(--violet-97);
}
.path-person span,
.path-person time {
  color: var(--muted);
  font-size: 12px;
}
.path-more {
  justify-self: start;
  margin: 4px 0 0;
  font-size: 12px;
}
@media (max-width: 760px) {
  .path-row {
    --indent: calc(var(--depth) * 12px);
  }
  .path-row-button {
    grid-template-columns: minmax(0, 1fr) 36px 40px;
    padding: 0 14px;
  }
  .path-bar {
    display: none;
  }
  .path-people {
    margin: 2px 14px 10px calc(14px + var(--indent));
  }
  .path-person {
    grid-template-columns: minmax(0, 1fr) auto;
  }
  .path-person time {
    display: none;
  }
}
</style>
