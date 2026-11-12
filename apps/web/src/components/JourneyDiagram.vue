<script setup lang="ts">
import { computed, ref } from 'vue';
import { labels, type Step, type Workflow } from '../lib/workflow';
import { portLabel, stepSummary } from '../lib/workflow-presentation';
const props = defineProps<{ workflow: Workflow; interactive?: boolean; selected?: string }>();
const emit = defineEmits<{ select: [string] }>();
const choices = ref<Record<string, string>>({});
const byID = computed(
  () =>
    new Map(props.workflow.nodes.map((node, index) => [node.id, { ...node, number: index + 1 }])),
);
const outgoing = (id: string) => props.workflow.edges.filter((edge) => edge.source === id);
const remaining = computed(() => {
  const memo = new Map<string, number>();
  const visiting = new Set<string>();
  function visit(id: string): number {
    if (memo.has(id)) return memo.get(id)!;
    if (visiting.has(id)) return 0;
    visiting.add(id);
    const size = 1 + Math.max(0, ...outgoing(id).map((edge) => visit(edge.target)));
    visiting.delete(id);
    memo.set(id, size);
    return size;
  }
  for (const node of props.workflow.nodes) visit(node.id);
  return memo;
});
const path = computed(() => {
  const rows: {
    node: Step & { number: number };
    next?: Workflow['edges'][number];
    alternatives: Workflow['edges'];
  }[] = [];
  let id = props.workflow.nodes.find((node) => node.type === 'event_trigger')?.id;
  const seen = new Set<string>();
  while (id && !seen.has(id) && rows.length < 100) {
    seen.add(id);
    const node = byID.value.get(id);
    if (!node) break;
    const edges = outgoing(id);
    const next =
      edges.find((edge) => edge.port === choices.value[id!]) ||
      [...edges].sort(
        (a, b) => (remaining.value.get(b.target) || 0) - (remaining.value.get(a.target) || 0),
      )[0];
    rows.push({ node, next, alternatives: edges.filter((edge) => edge.id !== next?.id) });
    id = next?.target;
  }
  return rows;
});
function choose(source: string, port: string) {
  choices.value = { ...choices.value, [source]: port };
}
const showAll = ref(false);
</script>
<template>
  <section class="journey-diagram" aria-label="Journey plan">
    <div class="diagram-context">
      <p v-if="workflow.global_exit">
        <strong>Global exit</strong> {{ workflow.global_exit.event_name }} → converted
      </p>
      <p>
        <strong>Reentry</strong>
        {{
          workflow.reentry?.mode === 'once'
            ? 'Once per customer'
            : workflow.reentry?.mode || 'After completion'
        }}
      </p>
    </div>
    <div class="diagram-heading">
      <span>{{ workflow.nodes.length }} steps · {{ workflow.edges.length }} connections</span
      ><button type="button" class="text-button" @click="showAll = !showAll">
        {{ showAll ? 'Show diagram' : 'All steps' }}
      </button>
    </div>
    <p class="diagram-hint">
      Design preview. Choose another branch to inspect its steps; this is not a test result.
    </p>
    <div v-if="showAll" class="diagram-all-steps">
      <button
        v-for="(node, index) in workflow.nodes"
        :key="node.id"
        type="button"
        class="diagram-node"
        @click="interactive && emit('select', node.id)"
      >
        <small>{{ index + 1 }} · {{ labels[node.type] }}</small
        ><strong>{{ node.label }}</strong
        ><span>{{ stepSummary(node) }}</span>
      </button>
    </div>
    <div v-else class="diagram-path">
      <div v-for="row in path" :key="row.node.id" class="diagram-row">
        <div class="diagram-main">
          <component
            :is="interactive ? 'button' : 'div'"
            :type="interactive ? 'button' : undefined"
            class="diagram-node"
            :class="[row.node.type, { 'diagram-selected': selected === row.node.id }]"
            @click="interactive && emit('select', row.node.id)"
            ><small>{{ row.node.number }} · {{ labels[row.node.type] }}</small
            ><strong>{{ row.node.label }}</strong
            ><span>{{ stepSummary(row.node) }}</span></component
          >
          <div v-if="row.next" class="diagram-arrow">
            <span>{{ outgoing(row.node.id).length > 1 ? portLabel(row.next.port) : '' }}</span
            ><span aria-hidden="true">↓</span>
          </div>
        </div>
        <div v-if="row.alternatives.length" class="diagram-branches">
          <button
            v-for="edge in row.alternatives"
            :key="edge.id"
            type="button"
            class="diagram-branch"
            @click="choose(row.node.id, edge.port)"
          >
            <span class="diagram-branch-label">{{ portLabel(edge.port) }} →</span
            ><strong>{{ byID.get(edge.target)?.label || edge.target }}</strong
            ><small>{{
              byID.get(edge.target)?.type === 'exit'
                ? stepSummary(byID.get(edge.target)!)
                : 'Follow this path →'
            }}</small>
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
