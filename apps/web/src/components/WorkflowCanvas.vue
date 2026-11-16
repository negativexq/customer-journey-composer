<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, provide, ref, watch } from 'vue';
import {
  VueFlow,
  setupVueFlow,
  Background,
  MiniMap,
  MarkerType,
  type Connection,
} from '@xyflow/vue';
import { layoutFlow } from '../lib/flow-layout';
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
import { stepCaptions } from '../lib/step-captions';
import { plural } from '../lib/format';
import InsertableEdge from './InsertableEdge.vue';
import StepInspector from './StepInspector.vue';
import InsertStepDialog from './InsertStepDialog.vue';
import AddStepDialog from './AddStepDialog.vue';
import StepLibrary from './StepLibrary.vue';
import CanvasToolbar from './CanvasToolbar.vue';
import AppIcon from './ui/AppIcon.vue';
import ShortcutsDialog from './ui/ShortcutsDialog.vue';
import { branchLabel, type CanvasEdge, type CanvasNode } from '../lib/step-catalog';
import '@xyflow/vue/style.css';
const props = defineProps<{
  modelValue: Workflow;
  readonly?: boolean;
  issues: Issue[];
  hideInspector?: boolean;
  aiOpen?: boolean;
}>();
const emit = defineEmits<{
  'update:modelValue': [Workflow];
  preview: [string];
  inspect: [];
  ai: [];
}>();
const {
  fitView,
  setViewport,
  setCenter,
  screenToFlowPosition,
  zoomIn,
  zoomOut,
  viewport,
  updateNodeInternals,
} = setupVueFlow<CanvasNode, CanvasEdge>();
const canvasElement = ref<HTMLDivElement>();
let initialViewportPlaced = false;
function placeInitialViewport() {
  if (initialViewportPlaced) return;
  const entry = nodes.value.find((node) => node.data.kind === 'event_trigger') || nodes.value[0];
  if (!entry || !canvasElement.value?.clientWidth) return;
  initialViewportPlaced = true;
  const width = canvasElement.value.clientWidth;
  const zoom = Math.min(0.9, (width - 24) / 224);
  // The first step is centered; when it would reach under the step count chip
  // at the top left, it starts below the chip instead.
  const chip = canvasElement.value.querySelector<HTMLElement>('.canvas-chip');
  const chipRight = chip ? chip.offsetLeft + chip.offsetWidth + 8 : 0;
  const top =
    width / 2 - 112 * zoom < chipRight
      ? (chip?.offsetTop ?? 0) + (chip?.offsetHeight ?? 0) + 16
      : 32;
  void setViewport({
    x: width / 2 - (entry.position.x + 112) * zoom,
    y: top - entry.position.y * zoom,
    zoom,
  });
}
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
    targetHandle: 'in',
    label: branchLabel(e.port),
  })),
);
const nodeLabels = computed(() => new Map(nodes.value.map((node) => [node.id, node.data.label])));
const shortExitLinks = ref(props.modelValue.nodes.length > 8);
const sharedExits = computed(() => {
  const count = new Map<string, number>();
  for (const edge of edges.value) count.set(edge.target, (count.get(edge.target) || 0) + 1);
  return new Set(
    nodes.value
      .filter((node) => node.data.kind === 'exit' && (count.get(node.id) || 0) > 1)
      .map((node) => node.id),
  );
});

const insertDialog = ref<InstanceType<typeof InsertStepDialog>>();
const insertionEdge = ref('');
function chooseEdge(id: string) {
  if (props.readonly) return;
  insertionEdge.value = id;
  insertDialog.value?.open();
}
function insertStep(kind: Kind, port: string) {
  if (props.readonly) return;
  const edge = edges.value.find((candidate) => candidate.id === insertionEdge.value);
  const source = nodes.value.find((node) => node.id === edge?.source);
  const target = nodes.value.find((node) => node.id === edge?.target);
  if (!edge || !source || !target) return;
  if (nodes.value.length >= 100 || edges.value.length >= 200) {
    configError.value = 'Workflow limit: 100 nodes and 200 connections';
    insertDialog.value?.close();
    return;
  }
  const id = crypto.randomUUID();
  nodes.value = [
    ...nodes.value,
    {
      id,
      type: 'step',
      position: {
        x: (source.position.x + target.position.x) / 2,
        y: (source.position.y + target.position.y) / 2,
      },
      data: { kind, label: labels[kind], config: defaults(kind) },
    },
  ];
  edges.value = edges.value.filter((candidate) => candidate.id !== edge.id);
  edges.value = [
    ...edges.value,
    { ...edge, target: id },
    {
      id: crypto.randomUUID(),
      source: id,
      target: target.id,
      sourceHandle: port,
      targetHandle: 'in',
      label: branchLabel(port),
    },
  ];
  selectedID.value = id;
  insertDialog.value?.close();
}
const selectedID = ref('');
// Steps are added from the canvas itself; the library opens on request.
const libraryOpen = ref(false);
const locked = ref(false);
const zoomLevel = computed(() => Math.round(viewport.value.zoom * 100));
const neighbors = computed(() => {
  if (!selectedID.value) return undefined;
  const ids = new Set([selectedID.value]);
  for (const edge of edges.value)
    if (edge.source === selectedID.value || edge.target === selectedID.value) {
      ids.add(edge.source);
      ids.add(edge.target);
    }
  return ids;
});
// Keyboard focus on a step outside the visible canvas pans the view to it. The flow box
// clips its overflow, so the browser cannot scroll it out of line with the viewport.
function revealFocusedNode(event: FocusEvent) {
  // A step's own buttons, like its "Add a step" button, bring the whole step into view.
  const focused = event.target as HTMLElement | null;
  const target = focused?.closest<HTMLElement>('.vue-flow__node');
  const pane = canvasElement.value?.querySelector('.vue-flow');
  if (!focused || !target || !pane) return;
  // The add button hangs below its card, so the step and the focused control are kept in view.
  const card = target.getBoundingClientRect();
  const own = focused.getBoundingClientRect();
  const box = {
    left: Math.min(card.left, own.left),
    right: Math.max(card.right, own.right),
    top: Math.min(card.top, own.top),
    bottom: Math.max(card.bottom, own.bottom),
  };
  const view = pane.getBoundingClientRect();
  const margin = 24;
  let dx = 0;
  let dy = 0;
  if (box.left < view.left + margin) dx = view.left + margin - box.left;
  else if (box.right > view.right - margin) dx = view.right - margin - box.right;
  // The canvas toolbar floats over the bottom of the view; a step must clear it too.
  const bar = canvasElement.value?.querySelector('.canvas-bar')?.getBoundingClientRect();
  const bottom = Math.min(view.bottom - margin, bar && bar.height ? bar.top - 16 : Infinity);
  if (box.top < view.top + margin) dy = view.top + margin - box.top;
  else if (box.bottom > bottom) dy = bottom - box.bottom;
  if (!dx && !dy) return;
  void setViewport({
    x: viewport.value.x + dx,
    y: viewport.value.y + dy,
    zoom: viewport.value.zoom,
  });
}
function focusNode(id: string) {
  const node = nodes.value.find((node) => node.id === id);
  if (!node) return;
  selectedID.value = id;
  nodes.value = nodes.value.map((node) => ({ ...node, selected: node.id === id }));
  inspectorOpen.value = true;
  emit('inspect');
  void nextTick(() =>
    setCenter(node.position.x + 112, node.position.y + 65, {
      zoom: 0.9,
      duration: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 160,
    }),
  );
}
function startView() {
  nodes.value = nodes.value.map((node) => ({ ...node, selected: false }));
  selectedID.value = '';
  inspectorOpen.value = false;
  initialViewportPlaced = false;
  void nextTick(placeInitialViewport);
}

const inspectorOpen = ref(true);

const selected = computed(() => nodes.value.find((n) => n.id === selectedID.value));
const captions = computed(() => stepCaptions(document.value));
// Outputs that lead nowhere yet; the card offers to continue from each.
const openPorts = computed(() => {
  const taken = new Set(
    edges.value.map((edge) => edge.source + ':' + (edge.sourceHandle || 'next')),
  );
  return new Map(
    nodes.value.map((node) => [
      node.id,
      ports(node.data.kind, node.data.config).filter((port) => !taken.has(node.id + ':' + port)),
    ]),
  );
});
const addDialog = ref<InstanceType<typeof AddStepDialog>>();
const addFrom = ref<{ node: string; port: string }>();
const addTitle = computed(() => {
  const source = nodes.value.find((node) => node.id === addFrom.value?.node);
  const branch = branchLabel(addFrom.value?.port || '');
  return source ? `After ${source.data.label}${branch ? ' · ' + branch : ''}` : '';
});
function chooseNext(node: string, port: string) {
  if (props.readonly) return;
  addFrom.value = { node, port };
  addDialog.value?.open();
}
provide('canvas-add-step', props.readonly ? undefined : chooseNext);
function addNext(kind: Kind) {
  const from = addFrom.value;
  const source = nodes.value.find((node) => node.id === from?.node);
  addDialog.value?.close();
  if (!from || !source || props.readonly) return;
  if (nodes.value.length >= 100 || edges.value.length >= 200) {
    configError.value = 'Workflow limit: 100 nodes and 200 connections';
    return;
  }
  const outputs = ports(source.data.kind, source.data.config);
  const card = canvasElement.value?.querySelector<HTMLElement>(
    `[data-id="${CSS.escape(source.id)}"] .workflow-node`,
  );
  // Branches fan out under their own handle; a single output continues straight down.
  const shift = (outputs.indexOf(from.port) - (outputs.length - 1) / 2) * 300;
  const id = crypto.randomUUID();
  nodes.value = [
    ...nodes.value.map((node) => ({ ...node, selected: false })),
    {
      id,
      type: 'step',
      position: {
        x: source.position.x + shift,
        y: source.position.y + (card?.offsetHeight || 130) + 96,
      },
      data: { kind, label: labels[kind], config: defaults(kind) },
    },
  ];
  edges.value = [
    ...edges.value,
    {
      id: crypto.randomUUID(),
      source: source.id,
      target: id,
      sourceHandle: from.port,
      targetHandle: 'in',
      label: branchLabel(from.port),
    },
  ];
  selectedID.value = id;
}
function patchConfig(patch: Partial<Config>) {
  if (props.readonly) return;
  const id = selectedID.value;
  nodes.value = nodes.value.map((node) =>
    node.id === id
      ? { ...node, data: { ...node.data, config: { ...node.data.config, ...patch } } }
      : node,
  );
  // A switch gains or loses output handles with its branches; have the canvas measure them again.
  if ('branches' in patch) void nextTick(() => updateNodeInternals([id]));
}
function rename(label: string) {
  if (props.readonly) return;
  nodes.value = nodes.value.map((node) =>
    node.id === selectedID.value ? { ...node, data: { ...node.data, label } } : node,
  );
}
// Branch outputs are positional, so removing a branch renumbers the connections after it.
function removeBranch(index: number) {
  const step = selected.value;
  const branches = step?.data.config.branches || [];
  if (!step || branches.length <= 2) return;
  const gone = `case_${index + 1}`;
  edges.value = edges.value
    .filter((edge) => !(edge.source === step.id && edge.sourceHandle === gone))
    .map((edge) => {
      const position = edge.sourceHandle?.startsWith('case_')
        ? Number(edge.sourceHandle.slice(5))
        : 0;
      return edge.source === step.id && position > index + 1
        ? {
            ...edge,
            sourceHandle: `case_${position - 1}`,
            label: branchLabel(`case_${position - 1}`),
          }
        : edge;
    });
  patchConfig({ branches: branches.filter((_, position) => position !== index) });
}
// The event a trigger listened for before it became an audience, kept while settings close.
const lastEventName = ref('');
watch(selectedID, () => (inspectorOpen.value = true));
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
    nodes.value = nodes.value.map((node) => ({
      ...node,
      class: issues.some((issue) => issue.node_id === node.id) ? 'invalid-node' : '',
    }));
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
    targetHandle: 'in',
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
  nodes.value = [
    ...nodes.value,
    {
      id,
      type: 'step',
      position,
      data: { kind, label: labels[kind], config: defaults(kind) },
    },
  ];
  selectedID.value = id;
}
function connect(connection: Connection) {
  if (props.readonly || !connection.source || !connection.target) return;
  edges.value = [
    ...edges.value,
    {
      id: crypto.randomUUID(),
      source: connection.source,
      target: connection.target,
      sourceHandle: connection.sourceHandle,
      targetHandle: 'in',
      label: branchLabel(connection.sourceHandle || 'next'),
    },
  ];
}
function drop(event: DragEvent) {
  event.preventDefault();
  const kind = event.dataTransfer?.getData('application/journey-node') as Kind;
  if (kinds.includes(kind)) add(kind, screenToFlowPosition({ x: event.clientX, y: event.clientY }));
}
function chosenNodes() {
  const selected = nodes.value.filter((n) => n.selected);
  return selected.length ? selected : nodes.value.filter((n) => n.id === selectedID.value);
}
function selectAll() {
  if (props.readonly) return;
  nodes.value = nodes.value.map((node) => ({ ...node, selected: true }));
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
  const added = copied.nodes.map((n) => ({
    ...n,
    id: ids.get(n.id)!,
    position: { x: n.position.x + 40, y: n.position.y + 40 },
    selected: true,
  }));
  nodes.value = [...nodes.value.map((n) => ({ ...n, selected: false })), ...added];
  edges.value = [
    ...edges.value,
    ...copied.edges.map((e) => ({
      ...e,
      id: crypto.randomUUID(),
      source: ids.get(e.source)!,
      target: ids.get(e.target)!,
    })),
  ];
  selectedID.value = added[0]?.id || '';
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
  const positions = layoutFlow(
    nodes.value.map((node) => {
      const card = canvasElement.value?.querySelector<HTMLElement>(
        `[data-id="${CSS.escape(node.id)}"] .workflow-node`,
      );
      return { id: node.id, width: card?.offsetWidth || 224, height: card?.offsetHeight || 130 };
    }),
    edges.value.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
      port: edge.sourceHandle || undefined,
    })),
  );
  nodes.value = nodes.value.map((node) => ({ ...node, position: positions[node.id]! }));
  void nextTick(startView);
}
const shortcutsOpen = ref(false);
function keyboard(event: KeyboardEvent) {
  const target = event.target as HTMLElement;
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable) return;
  if (
    event.key === '?' &&
    !event.metaKey &&
    !event.ctrlKey &&
    !window.document.querySelector('dialog[open]')
  ) {
    event.preventDefault();
    shortcutsOpen.value = true;
    return;
  }
  if (props.readonly) return;
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
</script>
<template>
  <InsertStepDialog ref="insertDialog" @insert="insertStep" />
  <AddStepDialog ref="addDialog" :title="addTitle" @choose="addNext" />
  <ShortcutsDialog v-model="shortcutsOpen" />
  <div class="builder-layout">
    <StepLibrary v-show="libraryOpen" :readonly="readonly" @add="add" />
    <div
      ref="canvasElement"
      class="canvas"
      @drop="drop"
      @dragover.prevent
      @focusin="revealFocusedNode"
    >
      <VueFlow
        id="builder"
        v-model:nodes="nodes"
        v-model:edges="edges"
        :default-edge-options="{
          type: 'insertable',
          markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16, color: 'var(--edge)' },
        }"
        :nodes-draggable="!readonly && !locked"
        :nodes-connectable="!readonly && !locked"
        :elements-selectable="true"
        :delete-key-code="readonly ? null : ['Backspace', 'Delete']"
        :multi-selection-key-code="['Meta', 'Control']"
        :default-viewport="{ x: 30, y: 35, zoom: 0.85 }"
        @nodes-initialized="placeInitialViewport"
        @connect="connect"
        @pane-click="selectedID = ''"
        @node-click="
          selectedID = $event.node.id;
          inspectorOpen = true;
          emit('inspect');
        "
        ><Background :gap="20" pattern-color="var(--canvas-dots)" /><MiniMap /><template
          #node-step="nodeProps"
          ><WorkflowNode
            v-bind="nodeProps"
            :data="{
              ...nodeProps.data,
              invalid: issues.some((i) => i.node_id === nodeProps.id),
              muted: !!neighbors && !neighbors.has(nodeProps.id),
              caption: captions[nodeProps.id],
              openPorts: readonly ? [] : openPorts.get(nodeProps.id),
            }"
        /></template>
        <template #edge-insertable="edgeProps"
          ><InsertableEdge
            v-bind="edgeProps"
            :connection-label="`${nodeLabels.get(edgeProps.source)} → ${nodeLabels.get(edgeProps.target)}${edgeProps.label ? ' · ' + edgeProps.label : ''}`"
            :readonly="readonly"
            :exit-reference="
              shortExitLinks &&
              sharedExits.has(edgeProps.target) &&
              Math.abs(edgeProps.targetY - edgeProps.sourceY) > 400
                ? nodeLabels.get(edgeProps.target)
                : undefined
            "
            :highlighted="
              !!selectedID && (edgeProps.source === selectedID || edgeProps.target === selectedID)
            "
            :muted="
              !!selectedID && edgeProps.source !== selectedID && edgeProps.target !== selectedID
            "
            @jump="focusNode(edgeProps.target)"
            @insert="chooseEdge(edgeProps.id)"
        /></template>
      </VueFlow>
      <div v-if="!nodes.length" class="canvas-empty">
        <p>Every journey starts with the event that brings a customer in.</p>
        <button v-if="!readonly" class="primary" type="button" @click="add('event_trigger')">
          <AppIcon name="plus" :size="15" />Add a trigger
        </button>
      </div>
      <div class="canvas-chip">
        <AppIcon name="workflow" :size="15" /><strong>{{ plural(nodes.length, 'step') }}</strong>
      </div>
      <CanvasToolbar
        v-model:short-exit-links="shortExitLinks"
        :readonly="readonly"
        :ai-open="aiOpen"
        :library-open="libraryOpen"
        :zoom-level="zoomLevel"
        :has-selection="!!selected"
        :inspector-shown="!!selected && inspectorOpen && !hideInspector"
        :locked="locked"
        :has-shared-exits="sharedExits.size > 0"
        @toggle-library="libraryOpen = !libraryOpen"
        @ai="emit('ai')"
        @undo="undo"
        @redo="redo"
        @zoom-in="zoomIn()"
        @zoom-out="zoomOut()"
        @fit="fitView({ padding: { top: '56px', right: '40px', bottom: '104px', left: '40px' } })"
        @toggle-inspector="inspectorOpen = !inspectorOpen"
        @layout="layout"
        @start="startView"
        @focus-selected="focusNode(selectedID)"
        @select-all="selectAll"
        @copy="copy"
        @paste="paste"
        @duplicate="duplicate"
        @toggle-lock="locked = !locked"
        @shortcuts="shortcutsOpen = true"
      />
    </div>
    <StepInspector
      v-if="selected && !hideInspector && inspectorOpen"
      v-model:last-event-name="lastEventName"
      :step="selected"
      :nodes="nodes"
      :edges="edges"
      :issues="issues"
      :readonly="readonly"
      :error="configError"
      @patch="patchConfig"
      @rename="rename"
      @remove-branch="removeBranch"
      @error="configError = $event"
      @close="selectedID = ''"
      @focus="focusNode"
      @add-next="chooseNext"
      @remove="remove"
      @preview="emit('preview', $event)"
    />
  </div>
</template>
