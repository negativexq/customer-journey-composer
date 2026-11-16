<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import {
  VueFlow,
  setupVueFlow,
  Background,
  MiniMap,
  MarkerType,
  type Node,
  type Edge,
} from '@xyflow/vue';
import { layoutFlow } from '../lib/flow-layout';
import { stepCaptions } from '../lib/step-captions';
import WorkflowNode from './WorkflowNode.vue';
import CanvasControls from './CanvasControls.vue';
import type { Workflow } from '../lib/workflow';
import { metric, type Insights } from '../lib/insights';
import { portLabel } from '../lib/workflow-presentation';
const props = defineProps<{
  workflow: Workflow;
  insights?: Insights;
  selectedNode?: string;
  // One person's route: step counts are left out because they describe everyone.
  personal?: boolean;
  execution?: {
    state: string;
    current_node_id: string;
    steps: { node_id: string; output: { port?: string } }[];
  };
}>();
const emit = defineEmits<{ select: [string] }>();
type MetricNode = Node<InstanceType<typeof WorkflowNode>['$props']['data']>;
const flowID = 'journey-metrics-' + crypto.randomUUID();
const { setViewport, fitView } = setupVueFlow<MetricNode, Edge>({ id: flowID });
const root = ref<HTMLDivElement>();
let placed = false;
const positions = computed(() =>
  layoutFlow(
    props.workflow.nodes.map((n) => ({ id: n.id, width: 248, height: 180 })),
    props.workflow.edges,
    { nodesep: 90, ranksep: 64, margin: 30 },
  ),
);
const value = (node: string, name: string, dim = 'node') => metric(props.insights, dim, name, node);
function observations(n: Workflow['nodes'][number]) {
  const items =
    n.type === 'email' || n.type === 'sms'
      ? [
          { label: 'Accepted', value: value(n.id, 'ACCEPTED', 'node_delivery') },
          ...(n.type === 'email'
            ? [{ label: 'Opened', value: value(n.id, 'opened', 'node_delivery') }]
            : []),
          { label: 'Clicked', value: value(n.id, 'clicked', 'node_delivery') },
          // Problems show only when there are some, so engagement fits on the card.
          ...[
            { label: 'Blocked', value: value(n.id, 'BLOCKED', 'node_delivery') },
            { label: 'Failed', value: value(n.id, 'FAILED', 'node_delivery') },
            { label: 'Bounced', value: value(n.id, 'bounced', 'node_delivery') },
          ].filter((item) => item.value > 0),
        ]
      : n.type === 'wait' || n.type === 'wait_event'
        ? [
            { label: 'Waiting now', value: value(n.id, 'WAITING', 'node_current') },
            { label: 'Reached', value: value(n.id, 'entered') },
          ]
        : [
            { label: 'Reached', value: value(n.id, 'entered') },
            { label: 'Completed step', value: value(n.id, 'SUCCEEDED') },
          ];
  const exits =
    props.insights?.metrics
      .filter((m) => m.dimension === 'node_exit' && m.key === n.id)
      .reduce((s, m) => s + m.value, 0) || 0;
  if (exits) items.push({ label: 'Exited here', value: exits });
  return items;
}

const captions = computed(() => stepCaptions(props.workflow));
const nodes = computed<MetricNode[]>(() =>
  props.workflow.nodes.map((n) => ({
    id: n.id,
    type: 'journey-step',
    position: positions.value[n.id]!,
    selected: props.selectedNode === n.id,
    data: {
      kind: n.type,
      label: n.label,
      config: n.config,
      caption: captions.value[n.id],
      active:
        props.execution?.current_node_id === n.id &&
        ['RUNNING', 'WAITING'].includes(props.execution.state),
      visited: props.execution?.steps.some((s) => s.node_id === n.id),
      muted:
        !!props.execution &&
        !props.execution.steps.some((s) => s.node_id === n.id) &&
        props.execution.current_node_id !== n.id,
      observations: props.personal ? undefined : observations(n),
      metrics: props.personal
        ? undefined
        : {
            entered: value(n.id, 'entered'),
            waiting: value(n.id, 'WAITING', 'node_current'),
            completed: value(n.id, 'SUCCEEDED'),
            failed: value(n.id, 'FAILED'),
            skipped: value(n.id, 'SKIPPED'),
            average: '—',
          },
    },
  })),
);
// The busiest connection sets the scale, so line weight reads as share of traffic.
const busiest = computed(() =>
  Math.max(
    0,
    ...props.workflow.edges.map((e) =>
      metric(props.insights, 'edge', 'traversed', e.source + ':' + e.port),
    ),
  ),
);
const edges = computed<Edge[]>(() =>
  props.workflow.edges.map((e) => {
    const visited = props.execution?.steps.some(
      (s) => s.node_id === e.source && s.output.port === e.port,
    );
    const count = metric(props.insights, 'edge', 'traversed', e.source + ':' + e.port);
    const total = props.workflow.edges
      .filter((x) => x.source === e.source)
      .reduce(
        (s, x) => s + metric(props.insights, 'edge', 'traversed', x.source + ':' + x.port),
        0,
      );
    const branching = props.workflow.edges.filter((x) => x.source === e.source).length > 1;
    return {
      id: e.id,
      source: e.source,
      target: e.target,
      sourceHandle: e.port,
      targetHandle: 'in',
      type: 'smoothstep',
      pathOptions: { offset: 26, borderRadius: 8 },
      label: branching
        ? `${portLabel(e.port)} · ${count}${total ? ' (' + Math.round((count / total) * 100) + '%)' : ''}`
        : '',
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: visited ? 'var(--accent-text)' : 'var(--edge)',
      },
      style: {
        stroke: visited
          ? 'var(--accent-text)'
          : count && !props.execution
            ? 'var(--edge)'
            : 'var(--edge-quiet)',
        strokeWidth: visited
          ? 2.5
          : props.execution || !busiest.value
            ? 1.5
            : 1.5 + 3 * (count / busiest.value),
        opacity: props.execution && !visited ? 0.35 : 1,
      },
    };
  }),
);
function start() {
  const n = props.workflow.nodes.find((n) => n.type === 'event_trigger') || props.workflow.nodes[0];
  if (!n || !root.value) return;
  const p = positions.value[n.id]!;
  const zoom = Math.min(0.9, (root.value.clientWidth - 24) / 248);
  void setViewport(
    // Start below the floating map actions so they never cover the first step.
    { x: root.value.clientWidth / 2 - (p.x + 124) * zoom, y: 56 - p.y * zoom, zoom },
    { duration: 160 },
  );
}
function initial() {
  if (!placed) {
    placed = true;
    // A map opened with a step already chosen, e.g. on returning to it, shows that step.
    if (props.selectedNode && positions.value[props.selectedNode]) focus(props.selectedNode);
    else start();
  }
}
function focus(id: string) {
  const p = positions.value[id];
  if (p && root.value)
    void setViewport(
      {
        x: root.value.clientWidth / 2 - (p.x + 124) * 0.9,
        y: root.value.clientHeight / 2 - (p.y + 85) * 0.9,
        zoom: 0.9,
      },
      { duration: 160 },
    );
}

watch(
  () => props.workflow,
  async () => {
    placed = false;
    await nextTick();
    initial();
  },
);
defineExpose({ focus, start });
</script>
<template>
  <div ref="root" class="monitor-canvas metrics-canvas">
    <div class="metrics-canvas-actions">
      <button @click="start">Go to start</button
      ><button :disabled="!selectedNode" @click="selectedNode && focus(selectedNode)">
        Focus step</button
      ><button
        @click="fitView({ padding: { top: '56px', right: '24px', bottom: '24px', left: '24px' } })"
      >
        Fit all
      </button>
    </div>
    <VueFlow
      :id="flowID"
      :nodes="nodes"
      :edges="edges"
      :nodes-draggable="false"
      :nodes-connectable="false"
      @nodes-initialized="initial"
      @node-click="({ node }) => emit('select', node.id)"
      ><Background /><CanvasControls /><MiniMap /><template #node-journey-step="nodeProps"
        ><WorkflowNode v-bind="nodeProps" /></template
    ></VueFlow>
  </div>
</template>
