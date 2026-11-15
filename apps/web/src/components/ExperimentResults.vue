<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { api } from '../lib/api';
import { errorMessage } from '../lib/format';
import type { Workflow } from '../lib/workflow';

// Side-by-side result of each A/B split on one published version: how many customers took each
// variant and how many of them reached the journey's goal.
const props = defineProps<{ journeyId: string; version: number; workflow: Workflow }>();
const emit = defineEmits<{ people: [{ node: string; port: string }] }>();
type Arm = {
  node_id: string;
  port: string;
  entered: number;
  converted: number;
  in_progress: number;
};
const arms = ref<Arm[]>([]);
const error = ref('');
let sequence = 0;

const splits = computed(() => props.workflow.nodes.filter((node) => node.type === 'split'));
const results = computed(() =>
  splits.value.map((node) => {
    const side = (port: string) => {
      const arm = arms.value.find((item) => item.node_id === node.id && item.port === port);
      const entered = arm?.entered || 0;
      const converted = arm?.converted || 0;
      return {
        port,
        name: port === 'a' ? 'Variant A' : 'Variant B',
        share: port === 'a' ? node.config.percent || 0 : 100 - (node.config.percent || 0),
        entered,
        converted,
        inProgress: arm?.in_progress || 0,
        rate: entered ? converted / entered : 0,
      };
    };
    const a = side('a');
    const b = side('b');
    // Too few customers to read anything into the difference.
    const thin = a.entered < 30 || b.entered < 30;
    const leader = thin || a.rate === b.rate ? undefined : a.rate > b.rate ? 'a' : 'b';
    return { node, sides: [a, b], thin, leader, widest: Math.max(a.rate, b.rate) };
  }),
);
const percent = (value: number) => (value * 100).toFixed(1) + '%';

async function load() {
  const current = ++sequence;
  error.value = '';
  if (!splits.value.length || !props.version) {
    arms.value = [];
    return;
  }
  try {
    const rows = await api<Arm[]>(
      `/journeys/${props.journeyId}/experiments?version=${props.version}`,
    );
    if (current === sequence) arms.value = rows;
  } catch (cause) {
    if (current === sequence) error.value = errorMessage(cause);
  }
}
watch(() => [props.journeyId, props.version, splits.value.length], load, { immediate: true });
onBeforeUnmount(() => sequence++);
defineExpose({ reload: load });
</script>
<template>
  <section v-if="splits.length" class="surface experiments" aria-label="Experiment results">
    <div class="surface-heading">
      <div>
        <h3>Experiments</h3>
        <p>Customers who took each variant, and how many of them reached the goal.</p>
      </div>
    </div>
    <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
    <article v-for="result in results" :key="result.node.id" class="experiment">
      <header>
        <strong>{{ result.node.label }}</strong>
        <span v-if="result.thin" class="experiment-note"
          >Fewer than 30 customers on a variant. Too early to compare.</span
        >
        <span v-else-if="!result.leader" class="experiment-note">The variants are level.</span>
        <span v-else class="experiment-note"
          >Variant {{ result.leader.toUpperCase() }} is ahead. This is a raw difference, not a
          significance test.</span
        >
      </header>
      <div class="experiment-sides">
        <div
          v-for="side in result.sides"
          :key="side.port"
          class="experiment-side"
          :class="{ leading: result.leader === side.port }"
        >
          <div class="side-title">
            <span class="side-name" :class="side.port">{{ side.name }}</span>
            <small>{{ side.share }}% of customers</small>
            <span v-if="result.leader === side.port" class="side-lead">Ahead</span>
          </div>
          <p class="side-rate">{{ side.entered ? percent(side.rate) : '—' }}</p>
          <div class="side-bar" aria-hidden="true">
            <i
              :style="{
                width: result.widest ? (side.rate / result.widest) * 100 + '%' : '0%',
              }"
            />
          </div>
          <dl>
            <div>
              <dt>Customers</dt>
              <dd>
                <button
                  v-if="side.entered"
                  type="button"
                  class="metric-link"
                  :aria-label="`Show ${side.entered} executions on ${side.name} of ${result.node.label}`"
                  @click="emit('people', { node: result.node.id, port: side.port })"
                >
                  {{ side.entered }}</button
                ><template v-else>0</template>
              </dd>
            </div>
            <div>
              <dt>Reached the goal</dt>
              <dd>{{ side.converted }}</dd>
            </div>
            <div>
              <dt>Still in progress</dt>
              <dd>{{ side.inProgress }}</dd>
            </div>
          </dl>
        </div>
      </div>
    </article>
  </section>
</template>
<style scoped>
.experiments {
  margin: 16px 0;
}
.experiment {
  padding: 16px 18px;
  border-top: 1px solid var(--border);
}
.experiment header {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
  margin-bottom: 12px;
}
.experiment header strong {
  font-size: 14px;
}
.experiment-note {
  font-size: 12px;
  color: var(--muted);
}
.experiment-sides {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.experiment-side {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
}
.experiment-side.leading {
  border-color: var(--accent);
}
.side-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.side-name {
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
}
.side-name.a {
  background: var(--accent);
  color: var(--on-solid);
}
.side-name.b {
  background: var(--accent-soft);
  color: var(--accent-text);
}
.side-title small {
  font-size: 12px;
  color: var(--muted);
}
.side-lead {
  margin-left: auto;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-text);
}
.side-rate {
  margin: 10px 0 6px;
  font-size: 26px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.side-bar {
  height: 6px;
  border-radius: 3px;
  background: var(--page);
  overflow: hidden;
}
.side-bar i {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--accent);
}
dl {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 12px 0 0;
}
dt {
  font-size: 12px;
  color: var(--muted);
}
dd {
  margin: 2px 0 0;
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
@media (max-width: 700px) {
  .experiment-sides {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
