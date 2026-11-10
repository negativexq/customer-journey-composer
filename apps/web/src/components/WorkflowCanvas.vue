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
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import '@vue-flow/minimap/dist/style.css';
const props = defineProps<{ modelValue: Workflow; readonly?: boolean; issues: Issue[] }>();
const emit = defineEmits<{ 'update:modelValue': [Workflow]; preview: [string] }>();
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
    label: e.port,
  })),
);
const selectedID = ref('');
const selected = computed(() => nodes.value.find((n) => n.id === selectedID.value));
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
    label: e.port,
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
function copy() {
  const chosen = nodes.value.filter((n) => n.selected || n.id === selectedID.value);
  clipboard = JSON.stringify(chosen);
}
function paste() {
  if (!clipboard || props.readonly) return;
  const copied: CanvasNode[] = JSON.parse(clipboard);
  for (const n of copied) {
    n.id = crypto.randomUUID();
    n.position = { x: n.position.x + 40, y: n.position.y + 40 };
    n.selected = false;
    nodes.value.push(n);
  }
}
function duplicate() {
  copy();
  paste();
}
function remove() {
  if (props.readonly || !selectedID.value) return;
  const id = selectedID.value;
  nodes.value = nodes.value.filter((n) => n.id !== id);
  edges.value = edges.value.filter((e) => e.source !== id && e.target !== id);
  selectedID.value = '';
}
function layout() {
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({ rankdir: 'TB', nodesep: 80, ranksep: 70 });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes.value) graph.setNode(n.id, { width: 190, height: 100 });
  for (const e of edges.value) graph.setEdge(e.source, e.target);
  dagre.layout(graph);
  for (const n of nodes.value) {
    const p = graph.node(n.id);
    n.position = { x: p.x - 95, y: p.y - 50 };
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
    <div class="node-library">
      <button
        v-for="kind in kinds"
        :key="kind"
        :draggable="!readonly"
        :disabled="readonly"
        @dragstart="drag($event, kind)"
        @click="add(kind)"
      >
        + {{ labels[kind] }}
      </button>
    </div>
    <div class="toolbar">
      <button :disabled="readonly" @click="undo">Undo</button
      ><button :disabled="readonly" @click="redo">Redo</button
      ><button :disabled="readonly" @click="duplicate">Duplicate node</button
      ><button :disabled="readonly" @click="layout">Auto layout</button
      ><button @click="fitView()">Fit view</button>
    </div>
  </div>
  <div class="builder-layout">
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
        fit-view-on-init
        @update:nodes="updateNodes"
        @update:edges="updateEdges"
        @connect="connect"
        @node-click="selectedID = $event.node.id"
        ><Background :gap="20" pattern-color="#d6dfd1" /><Controls /><MiniMap /><template
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
    <aside v-if="selected" class="node-config">
      <h3>{{ labels[selected.data.kind] }}</h3>
      <label>Node label<input v-model="selected.data.label" :disabled="readonly" /></label
      ><label v-if="['event_trigger', 'wait_event'].includes(selected.data.kind)"
        >Event name<input v-model="selected.data.config.event_name" :disabled="readonly" /></label
      ><label v-if="selected.data.kind === 'wait'"
        >Delay (seconds)<input
          v-model.number="selected.data.config.duration_seconds"
          type="number"
          min="1"
          :disabled="readonly" /></label
      ><label v-if="selected.data.kind === 'wait'"
        >Timezone<input v-model="selected.data.config.timezone" :disabled="readonly" /></label
      ><label v-if="selected.data.kind === 'wait_event'"
        >Timeout (seconds)<input
          v-model.number="selected.data.config.timeout_seconds"
          type="number"
          min="1"
          :disabled="readonly" /></label
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
  </div>
</template>
