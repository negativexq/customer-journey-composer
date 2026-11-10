<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { VueFlow, useVueFlow, type Node, type Edge, type Connection } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { MiniMap } from '@vue-flow/minimap';
import dagre from '@dagrejs/dagre';
import {
  kinds,
  defaults,
  ports,
  labels,
  type Kind,
  type Workflow,
  type Config,
  type Issue,
} from '../lib/workflow';
import WorkflowNode from './WorkflowNode.vue';
import RuleEditor from './RuleEditor.vue';
import AppIcon from './ui/AppIcon.vue';
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import '@vue-flow/minimap/dist/style.css';
const props = defineProps<{
  modelValue: Workflow;
  readonly?: boolean;
  issues: Issue[];
  hideInspector?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [Workflow]; preview: [string]; inspect: [] }>();
const { fitView, screenToFlowCoordinate } = useVueFlow('builder');
type CanvasData = { kind: Kind; label: string; config: Config };
type CanvasNode = {
  id: string;
  type: string;
  position: { x: number; y: number };
  data: CanvasData;
  selected?: boolean;
  class?: string;
};
type CanvasEdge = {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string | null;
  label?: string;
};
const branchLabel = (port: string) =>
  ({ true: 'Yes', false: 'No', matched: 'Matched', timeout: 'Timeout' })[port] || '';
const nodes = ref<CanvasNode[]>(
  props.modelValue.nodes.map((n) => ({
    id: n.id,
    type: 'step',
    position: n.position,
    data: { kind: n.type, label: n.label, config: JSON.parse(JSON.stringify(n.config)) as Config },
  })),
);
const edges = ref<CanvasEdge[]>(
  props.modelValue.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    sourceHandle: e.port,
    label: branchLabel(e.port),
  })),
);
const selectedID = ref('');
const libraryOpen = ref(true);
const inspectorOpen = ref(true);

const nodeSearch = ref('');
const iconFor: Record<string, string> = {
  event_trigger: 'zap',
  wait: 'clock',
  wait_event: 'events',
  condition: 'branch',
  email: 'email',
  sms: 'message',
  update_attribute: 'user',
  exit: 'exit',
};
const descriptions: Record<string, string> = {
  event_trigger: 'Start from a customer event',
  wait: 'Wait before the next step',
  wait_event: 'Listen for an event',
  condition: 'Split by a condition',
  email: 'Send an email message',
  sms: 'Send a text message',
  update_attribute: 'Change a customer attribute',
  exit: 'Complete this journey',
};
const visibleKinds = computed(() =>
  kinds.filter((k) => labels[k].toLowerCase().includes(nodeSearch.value.toLowerCase())),
);
const paletteGroups = computed(() =>
  [
    { label: 'Entry', kinds: ['event_trigger'] },
    { label: 'Logic & timing', kinds: ['condition', 'wait', 'wait_event'] },
    { label: 'Messages', kinds: ['email', 'sms'] },
    { label: 'Customer & outcomes', kinds: ['update_attribute', 'exit'] },
  ]
    .map((group) => ({
      ...group,
      kinds: group.kinds.filter((kind) => visibleKinds.value.includes(kind as Kind)) as Kind[],
    }))
    .filter((group) => group.kinds.length),
);
const selected = computed(() => nodes.value.find((n) => n.id === selectedID.value));
const delayUnit = ref(3600);
const delayKey = computed(() =>
  selected.value?.data.kind === 'wait_event' ? 'timeout_seconds' : 'duration_seconds',
);
const delayAmount = computed({
  get: () => (selected.value?.data.config[delayKey.value] || 0) / delayUnit.value,
  set: (value: number) => {
    if (selected.value)
      selected.value.data.config[delayKey.value] = Math.round(Number(value) * delayUnit.value);
  },
});
watch(selectedID, () => {
  inspectorOpen.value = true;
  const seconds = selected.value?.data.config[delayKey.value] || 0;
  delayUnit.value = [86400, 3600, 60].find((unit) => seconds > 0 && seconds % unit === 0) || 1;
});
function changeDelayUnit(value: number) {
  const amount = delayAmount.value;
  delayUnit.value = value;
  delayAmount.value = amount;
}
const history: string[] = [];
const future: string[] = [];
let restoring = false;
let clipboard = '';
const configError = ref('');
const document = computed<Workflow>(() => ({
  ...props.modelValue,
  schema_version: 1,
  nodes: nodes.value.map((n) => ({
    id: n.id,
    type: n.data.kind,
    label: n.data.label,
    position: { x: n.position.x, y: n.position.y },
    config: n.data.config,
  })),
  edges: edges.value.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    port: e.sourceHandle || 'next',
  })),
}));
let previous = JSON.stringify(document.value);
watch(
  () => JSON.stringify(document.value),
  (current) => {
    if (restoring) {
      previous = current;
      return;
    }
    if (previous === current) return;
    history.push(previous);
    if (history.length > 60) history.shift();
    future.length = 0;
    previous = current;
    emit('update:modelValue', JSON.parse(current));
  },
);
watch(
  () => props.issues,
  (issues) => {
    for (const node of nodes.value)
      node.class = issues.some((i) => i.node_id === node.id) ? 'invalid-node' : '';
  },
);
async function restore(data: string) {
  restoring = true;
  const w: Workflow = JSON.parse(data);
  nodes.value = w.nodes.map((n) => ({
    id: n.id,
    type: 'step',
    position: n.position,
    data: { kind: n.type, label: n.label, config: n.config },
  }));
  edges.value = w.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    sourceHandle: e.port,
    label: branchLabel(e.port),
  }));
  await nextTick();
  restoring = false;
  previous = data;
  emit('update:modelValue', w);
}
function undo() {
  const old = history.pop();
  if (old) {
    future.push(previous);
    void restore(old);
  }
}
function redo() {
  const next = future.pop();
  if (next) {
    history.push(previous);
    void restore(next);
  }
}
function add(
  kind: Kind,
  position = {
    x: 120 + (nodes.value.length % 3) * 230,
    y: 80 + Math.floor(nodes.value.length / 3) * 160,
  },
) {
  if (props.readonly) return;
  const id = crypto.randomUUID();
  nodes.value.push({
    id,
    type: 'step',
    position,
    data: { kind, label: labels[kind], config: defaults(kind) },
  });
  selectedID.value = id;
}
function updateNodes(value: Node[]) {
  nodes.value = value.map((n) => ({
    id: n.id,
    type: 'step',
    position: n.position,
    data: n.data as CanvasData,
    selected: (n as Node & { selected?: boolean }).selected,
    class: typeof n.class === 'string' ? n.class : undefined,
  }));
}
function updateEdges(value: Edge[]) {
  edges.value = value.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    sourceHandle: e.sourceHandle,
    label: typeof e.label === 'string' ? e.label : undefined,
  }));
}
function connect(connection: Connection) {
  if (props.readonly || !connection.source || !connection.target) return;
  edges.value.push({
    id: crypto.randomUUID(),
    source: connection.source,
    target: connection.target,
    sourceHandle: connection.sourceHandle,
    label: connection.sourceHandle || 'next',
  });
}
function drop(event: DragEvent) {
  event.preventDefault();
  const kind = event.dataTransfer?.getData('application/journey-node') as Kind;
  if (kinds.includes(kind))
    add(kind, screenToFlowCoordinate({ x: event.clientX, y: event.clientY }));
}
function drag(event: DragEvent, kind: Kind) {
  event.dataTransfer?.setData('application/journey-node', kind);
}
function chosenNodes() {
  const selected = nodes.value.filter((n) => n.selected);
  return selected.length ? selected : nodes.value.filter((n) => n.id === selectedID.value);
}
function selectAll() {
  if (props.readonly) return;
  for (const n of nodes.value) n.selected = true;
  selectedID.value = nodes.value[0]?.id || '';
}
function copy() {
  const chosen = chosenNodes();
  const ids = new Set(chosen.map((n) => n.id));
  clipboard = JSON.stringify({
    nodes: chosen,
    edges: edges.value.filter((e) => ids.has(e.source) && ids.has(e.target)),
  });
}
function paste() {
  if (!clipboard || props.readonly) return;
  const copied = JSON.parse(clipboard) as { nodes: CanvasNode[]; edges: CanvasEdge[] };
  if (
    nodes.value.length + copied.nodes.length > 100 ||
    edges.value.length + copied.edges.length > 200
  ) {
    configError.value = 'Workflow limit: 100 nodes and 200 connections';
    return;
  }
  const ids = new Map<string, string>();
  for (const n of copied.nodes) ids.set(n.id, crypto.randomUUID());
  for (const n of nodes.value) n.selected = false;
  for (const n of copied.nodes) {
    n.id = ids.get(n.id)!;
    n.position = { x: n.position.x + 40, y: n.position.y + 40 };
    n.selected = true;
    nodes.value.push(n);
  }
  for (const e of copied.edges)
    edges.value.push({
      ...e,
      id: crypto.randomUUID(),
      source: ids.get(e.source)!,
      target: ids.get(e.target)!,
    });
  selectedID.value = copied.nodes[0]?.id || '';
}
function duplicate() {
  copy();
  paste();
}
function remove() {
  if (props.readonly) return;
  const ids = new Set(chosenNodes().map((n) => n.id));
  nodes.value = nodes.value.filter((n) => !ids.has(n.id));
  edges.value = edges.value.filter((e) => !ids.has(e.source) && !ids.has(e.target));
  selectedID.value = '';
}
function layout() {
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({ rankdir: 'TB', nodesep: 80, ranksep: 70 });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes.value) graph.setNode(n.id, { width: 224, height: 112 });
  for (const e of edges.value) graph.setEdge(e.source, e.target);
  dagre.layout(graph);
  for (const n of nodes.value) {
    const p = graph.node(n.id);
    n.position = { x: p.x - 112, y: p.y - 56 };
  }
  void nextTick(() => fitView());
}
function keyboard(event: KeyboardEvent) {
  if (
    ['INPUT', 'TEXTAREA', 'SELECT'].includes((event.target as HTMLElement).tagName) ||
    props.readonly
  )
    return;
  if (event.metaKey || event.ctrlKey) {
    if (event.key === 'a') {
      event.preventDefault();
      selectAll();
    }
    if (event.key === 'z') {
      event.preventDefault();
      if (event.shiftKey) redo();
      else undo();
    }
    if (event.key === 'c') {
      event.preventDefault();
      copy();
    }
    if (event.key === 'v') {
      event.preventDefault();
      paste();
    }
    if (event.key === 'd') {
      event.preventDefault();
      duplicate();
    }
  }
}
window.addEventListener('keydown', keyboard);
onBeforeUnmount(() => window.removeEventListener('keydown', keyboard));
function setAttribute(value: string) {
  try {
    if (selected.value) selected.value.data.config.value = JSON.parse(value);
    configError.value = '';
  } catch {
    configError.value = 'Attribute value must be valid JSON';
  }
}
</script>
<template>
  <div class="builder-toolbar">
    <div class="canvas-title">
      <button
        class="icon-button"
        aria-label="Toggle step library"
        :aria-expanded="libraryOpen"
        @click="libraryOpen = !libraryOpen"
      >
        <AppIcon name="templates" :size="16" />
      </button>
      <AppIcon name="workflow" :size="17" /><strong>Workflow</strong
      ><span>{{ nodes.length }} steps</span>
    </div>
    <div class="canvas-tools">
      <button
        class="icon-button"
        aria-label="Toggle step settings"
        :aria-expanded="inspectorOpen && !hideInspector"
        @click="inspectorOpen = !inspectorOpen"
      >
        <AppIcon name="settings" :size="16" />
      </button>
      <button
        v-for="[title, icon, handler] in [
          ['Undo', 'undo', undo],
          ['Redo', 'redo', redo],
          ['Select all nodes', 'select', selectAll],
          ['Copy selection', 'copy', copy],
          ['Paste selection', 'paste', paste],
          ['Duplicate node', 'plus', duplicate],
          ['Auto layout', 'layout', layout],
        ] as const"
        :key="title"
        class="icon-button"
        :disabled="readonly"
        :aria-label="title"
        :title="title"
        @click="handler"
      >
        <AppIcon :name="icon" :size="16" /></button
      ><span class="toolbar-divider" /><button
        class="icon-button"
        aria-label="Fit view"
        title="Fit view"
        @click="fitView()"
      >
        <AppIcon name="maximize" :size="16" />
      </button>
    </div>
  </div>
  <div class="builder-layout">
    <aside v-show="libraryOpen" class="step-library" aria-label="Step library">
      <div class="step-library-heading">
        <h3>Elements</h3>
        <span>{{ kinds.length }} types</span>
      </div>
      <label class="step-search"
        ><AppIcon name="search" :size="14" /><input
          v-model="nodeSearch"
          aria-label="Search steps"
          placeholder="Find a step..."
      /></label>
      <div class="node-library">
        <section v-for="group in paletteGroups" :key="group.label" class="palette-group">
          <h4>{{ group.label }}</h4>
          <button
            v-for="kind in group.kinds"
            :key="kind"
            :draggable="!readonly"
            :disabled="readonly"
            :aria-label="'+ ' + labels[kind]"
            :class="'palette-' + kind"
            :title="descriptions[kind]"
            @dragstart="drag($event, kind)"
            @click="add(kind)"
          >
            <span class="step-type-icon" :class="kind"
              ><AppIcon :name="iconFor[kind]" :size="17" /></span
            ><span
              ><strong>{{ labels[kind] }}</strong
              ><small>{{ descriptions[kind] }}</small></span
            ><AppIcon name="plus" :size="13" />
          </button>
        </section>
        <p v-if="!paletteGroups.length" class="muted">No matching steps.</p>
      </div>
      <div class="library-tip">
        <AppIcon name="sparkles" :size="16" />
        <p>Prefer to describe it?<br />Try the AI composer.</p>
      </div>
    </aside>
    <div class="canvas" @drop="drop" @dragover.prevent>
      <VueFlow
        id="builder"
        :nodes="nodes"
        :edges="edges"
        :nodes-draggable="!readonly"
        :nodes-connectable="!readonly"
        :elements-selectable="true"
        :delete-key-code="readonly ? null : ['Backspace', 'Delete']"
        :multi-selection-key-code="['Meta', 'Control']"
        :default-viewport="{ x: 30, y: 35, zoom: 0.85 }"
        @update:nodes="updateNodes"
        @update:edges="updateEdges"
        @connect="connect"
        @node-click="
          selectedID = $event.node.id;
          inspectorOpen = true;
          emit('inspect');
        "
        ><Background :gap="20" pattern-color="#dde1ec" /><Controls /><MiniMap /><template
          #node-step="nodeProps"
          ><WorkflowNode
            v-bind="nodeProps"
            :data="{
              ...nodeProps.data,
              invalid: issues.some((i) => i.node_id === nodeProps.id),
            }" /></template
      ></VueFlow>
      <p v-if="!nodes.length" class="canvas-empty">Add an Event Trigger to start your journey.</p>
    </div>
    <aside
      v-if="selected && !hideInspector && inspectorOpen"
      class="node-config"
      aria-label="Step settings"
    >
      <div class="inspector-heading">
        <div>
          <span class="inspector-eyebrow">STEP SETTINGS</span>
          <h3>{{ labels[selected.data.kind] }}</h3>
        </div>
        <button class="icon-button" aria-label="Close step settings" @click="selectedID = ''">
          <AppIcon name="close" :size="17" />
        </button>
      </div>
      <label>Step name<input v-model="selected.data.label" :disabled="readonly" /></label
      ><label v-if="['event_trigger', 'wait_event'].includes(selected.data.kind)"
        >Event name<input v-model="selected.data.config.event_name" :disabled="readonly"
      /></label>
      <div v-if="['wait', 'wait_event'].includes(selected.data.kind)" class="duration-field">
        <label
          >{{ selected.data.kind === 'wait_event' ? 'Timeout' : 'Wait for'
          }}<input v-model.number="delayAmount" type="number" min="1" :disabled="readonly"
        /></label>
        <label
          >Unit<select
            :value="delayUnit"
            aria-label="Delay unit"
            :disabled="readonly"
            @change="changeDelayUnit(Number(($event.target as HTMLSelectElement).value))"
          >
            <option :value="1">Seconds</option>
            <option :value="60">Minutes</option>
            <option :value="3600">Hours</option>
            <option :value="86400">Days</option>
          </select></label
        >
      </div>
      <label v-if="selected.data.kind === 'wait'"
        >Timezone<input v-model="selected.data.config.timezone" :disabled="readonly" /></label
      ><RuleEditor
        v-if="selected.data.kind === 'condition'"
        v-model="selected.data.config.rule"
        :readonly="readonly"
      />
      <details v-if="['event_trigger', 'wait_event'].includes(selected.data.kind)">
        <summary>Event filters</summary>
        <button
          v-if="!selected.data.config.filters"
          :disabled="readonly"
          @click="
            selected.data.config.filters = {
              path: 'event.properties.amount',
              operator: 'greater_than',
              value: 0,
            }
          "
        >
          Add filter</button
        ><RuleEditor v-else v-model="selected.data.config.filters" :readonly="readonly" />
      </details>
      <p v-if="configError" role="alert">{{ configError }}</p>
      <template v-if="selected.data.kind === 'email'"
        ><label>Subject<input v-model="selected.data.config.subject" :disabled="readonly" /></label
        ><label
          >Preheader<input v-model="selected.data.config.preheader" :disabled="readonly" /></label
        ><label
          >HTML content<textarea
            v-model="selected.data.config.html"
            rows="8"
            :disabled="readonly"
          /></label></template
      ><label v-if="['email', 'sms'].includes(selected.data.kind)"
        >Message text<textarea
          v-model="selected.data.config.text"
          :disabled="readonly"
          rows="5"
        /></label
      ><template v-if="selected.data.kind === 'update_attribute'"
        ><label
          >Attribute<input v-model="selected.data.config.attribute" :disabled="readonly" /></label
        ><label
          >Operation<select v-model="selected.data.config.operation" :disabled="readonly">
            <option>set</option>
            <option>increment</option>
            <option>unset</option>
          </select></label
        ><label
          >Value (JSON)<input
            :value="JSON.stringify(selected.data.config.value)"
            :disabled="readonly"
            @change="setAttribute(($event.target as HTMLInputElement).value)" /></label></template
      ><label v-if="selected.data.kind === 'exit'"
        >Exit reason<select v-model="selected.data.config.reason" :disabled="readonly">
          <option>completed</option>
          <option>converted</option>
          <option>condition_not_met</option>
          <option>manually_cancelled</option>
        </select></label
      >
      <p
        v-for="issue in issues.filter((i) => i.node_id === selectedID)"
        :key="issue.field + issue.message"
        class="error-message"
      >
        {{ issue.message }}
      </p>
      <button
        v-if="['email', 'sms'].includes(selected.data.kind)"
        @click="emit('preview', selected.id)"
      >
        Preview / Test node</button
      ><button :disabled="readonly" @click="remove">Delete node</button
      ><small>Outputs: {{ ports(selected.data.kind).join(', ') || 'Completed' }}</small>
    </aside>
    <aside
      v-else-if="!hideInspector && inspectorOpen"
      class="node-config inspector-empty"
      aria-label="Journey inspector"
    >
      <span class="inspector-eyebrow">JOURNEY INSPECTOR</span>
      <span class="empty-inspector-icon"><AppIcon name="select" :size="26" /></span>
      <h3>Select a step</h3>
      <p>Choose a step on the canvas to edit its settings, conditions or message.</p>
      <div class="inspector-summary">
        <span><AppIcon name="workflow" :size="15" />{{ nodes.length }} steps</span
        ><span><AppIcon name="branch" :size="15" />{{ edges.length }} connections</span>
      </div>
      <p class="inspector-safety">
        <AppIcon name="shield" :size="16" />Edits stay in your draft until you review and publish a
        version.
      </p>
    </aside>
  </div>
</template>
