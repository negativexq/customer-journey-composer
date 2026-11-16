<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
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
  audienceEvent,
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
import { portLabel } from '../lib/workflow-presentation';
import { stepCaptions } from '../lib/step-captions';
import { engagementEvent, engagementEvents } from '../lib/engagement';
import { plural } from '../lib/format';
import InsertableEdge from './InsertableEdge.vue';
import RuleEditor from './RuleEditor.vue';
import AppIcon from './ui/AppIcon.vue';
import ShortcutsDialog from './ui/ShortcutsDialog.vue';
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
  type?: 'insertable';
  data?: Record<string, unknown>;
  id: string;
  source: string;
  target: string;
  sourceHandle?: string | null;
  targetHandle?: string;
  label?: string;
};
const branchLabel = (port: string) =>
  (
    ({
      true: 'Yes',
      false: 'No',
      matched: 'Matched',
      timeout: 'Timeout',
      a: 'A',
      b: 'B',
    }) as Record<string, string>
  )[port] || (port === 'else' ? 'Otherwise' : port.startsWith('case_') ? port.slice(5) : '');
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

const insertionDialog = ref<HTMLDialogElement>();
const insertionEdge = ref('');
const insertionPort = ref('');
const insertKind = ref<Kind>('wait');
const insertKinds = kinds.filter((kind) => kind !== 'event_trigger' && kind !== 'exit');
function chooseEdge(id: string) {
  if (props.readonly) return;
  insertionEdge.value = id;
  insertKind.value = 'wait';
  insertionPort.value = ports(insertKind.value)[0]!;
  insertionDialog.value?.showModal();
}
watch(insertKind, (kind) => {
  insertionPort.value = ports(kind).length > 1 ? '' : ports(kind)[0]!;
});
function insertStep() {
  if (props.readonly) return;
  const edge = edges.value.find((candidate) => candidate.id === insertionEdge.value);
  const source = nodes.value.find((node) => node.id === edge?.source);
  const target = nodes.value.find((node) => node.id === edge?.target);
  if (!edge || !source || !target || !ports(insertKind.value).includes(insertionPort.value)) return;
  if (nodes.value.length >= 100 || edges.value.length >= 200) {
    configError.value = 'Workflow limit: 100 nodes and 200 connections';
    insertionDialog.value?.close();
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
      data: {
        kind: insertKind.value,
        label: labels[insertKind.value],
        config: defaults(insertKind.value),
      },
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
      sourceHandle: insertionPort.value,
      targetHandle: 'in',
      label: branchLabel(insertionPort.value),
    },
  ];
  selectedID.value = id;
  insertionDialog.value?.close();
}
const selectedID = ref('');
// Steps are added from the canvas itself; the library opens on request.
const libraryOpen = ref(false);
const locked = ref(false);
const moreMenu = ref<HTMLDetailsElement>();
const zoomLevel = computed(() => Math.round(viewport.value.zoom * 100));
// Close the menu when the pointer goes anywhere else.
function closeMenuOutside(event: PointerEvent) {
  if (moreMenu.value?.open && !moreMenu.value.contains(event.target as Node))
    moreMenu.value.removeAttribute('open');
}
window.addEventListener('pointerdown', closeMenuOutside);
onBeforeUnmount(() => window.removeEventListener('pointerdown', closeMenuOutside));
function fromMenu(action: () => void) {
  moreMenu.value?.removeAttribute('open');
  action();
}
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
// What leads into the selected step and where each of its outputs goes.
const incomingSteps = computed(() =>
  edges.value
    .filter((edge) => edge.target === selectedID.value)
    .map((edge) => ({
      id: edge.id,
      source: edge.source,
      label: nodeLabels.value.get(edge.source) || edge.source,
      branch: branchLabel(edge.sourceHandle || ''),
    })),
);
const outgoingSteps = computed(() => {
  const step = nodes.value.find((node) => node.id === selectedID.value);
  if (!step) return [];
  return ports(step.data.kind, step.data.config).map((port) => {
    const edge = edges.value.find(
      (candidate) => candidate.source === step.id && (candidate.sourceHandle || 'next') === port,
    );
    return {
      port,
      name: portLabel(port),
      target: edge?.target,
      label: edge ? nodeLabels.value.get(edge.target) || edge.target : '',
    };
  });
});
const groupTitle: Record<string, string> = {
  event_trigger: 'Entry event',
  wait: 'Timing',
  wait_event: 'Event to wait for',
  condition: 'Rule',
  switch: 'Branches',
  split: 'Experiment',
  email: 'Email content',
  sms: 'Message',
  webhook: 'Request',
  update_attribute: 'Profile change',
  exit: 'Outcome',
};
// Keyboard focus on a step outside the visible canvas pans the view to it. The flow box
// clips its overflow, so the browser cannot scroll it out of line with the viewport.
function revealFocusedNode(event: FocusEvent) {
  const target = event.target as HTMLElement | null;
  const pane = canvasElement.value?.querySelector('.vue-flow');
  if (!target?.classList.contains('vue-flow__node') || !pane) return;
  const box = target.getBoundingClientRect();
  const view = pane.getBoundingClientRect();
  const margin = 24;
  let dx = 0;
  let dy = 0;
  if (box.left < view.left + margin) dx = view.left + margin - box.left;
  else if (box.right > view.right - margin) dx = view.right - margin - box.right;
  if (box.top < view.top + margin) dy = view.top + margin - box.top;
  else if (box.bottom > view.bottom - margin) dy = view.bottom - margin - box.bottom;
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

const nodeSearch = ref('');
const iconFor: Record<string, string> = {
  event_trigger: 'zap',
  wait: 'clock',
  wait_event: 'events',
  condition: 'branch',
  switch: 'branch',
  split: 'split',
  email: 'email',
  sms: 'message',
  webhook: 'webhook',
  update_attribute: 'user',
  exit: 'exit',
};
// Headings for the step-type menu in the insert dialog.
const stepGroup: Record<string, string> = {
  wait: 'Timing',
  wait_event: 'Timing',
  condition: 'Branching',
  switch: 'Branching',
  split: 'Branching',
  email: 'Actions',
  sms: 'Actions',
  webhook: 'Actions',
  update_attribute: 'Actions',
  exit: 'End',
};
// One sentence on what a step does, shown at the top of its settings.
const explanations: Record<string, string> = {
  event_trigger:
    'A customer enters each time the entry event arrives for them, or, with an audience, when they match it at the scheduled time.',
  wait: 'Holds the customer here for a set time, then moves on.',
  wait_event:
    'Watches for an event. The customer takes Matched if it arrives in time and Timeout if it does not.',
  condition: 'Checks the customer against a rule and sends them down Yes or No.',
  switch:
    'Checks the branches from top to bottom and sends the customer down the first one that matches, or Otherwise if none does.',
  split:
    'Sends a fixed share of customers to variant A and the rest to B. A customer always gets the same variant here.',
  email: 'Sends an email, if the customer allows email and the sending limit has room.',
  sms: 'Sends a text message, if the customer allows SMS and the sending limit has room.',
  webhook:
    'Posts the chosen customer and event fields to another system as JSON. Only hosts the server allows are called; others are recorded but not sent.',
  update_attribute: 'Changes a value on the customer profile for later steps and journeys.',
  exit: 'Ends the journey for this customer and records why.',
};
const descriptions: Record<string, string> = {
  event_trigger: 'Start from a customer event',
  wait: 'Wait before the next step',
  wait_event: 'Listen for an event',
  condition: 'Split by a condition',
  switch: 'Split into several paths',
  split: 'Test two variants',
  email: 'Send an email message',
  sms: 'Send a text message',
  webhook: 'Notify another system',
  update_attribute: 'Change a customer attribute',
  exit: 'Complete this journey',
};
const visibleKinds = computed(() =>
  kinds.filter((k) => labels[k].toLowerCase().includes(nodeSearch.value.toLowerCase())),
);
const paletteGroups = computed(() =>
  [
    { label: 'Entry', kinds: ['event_trigger'] },
    { label: 'Logic & timing', kinds: ['condition', 'switch', 'split', 'wait', 'wait_event'] },
    { label: 'Messages', kinds: ['email', 'sms'] },
    { label: 'Customer & outcomes', kinds: ['update_attribute', 'webhook', 'exit'] },
  ]
    .map((group) => ({
      ...group,
      kinds: group.kinds.filter((kind) => visibleKinds.value.includes(kind as Kind)) as Kind[],
    }))
    .filter((group) => group.kinds.length),
);
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
const addDialog = ref<HTMLDialogElement>();
const addFrom = ref<{ node: string; port: string }>();
const addKinds = kinds.filter((kind) => kind !== 'event_trigger');
const addTitle = computed(() => {
  const source = nodes.value.find((node) => node.id === addFrom.value?.node);
  const branch = branchLabel(addFrom.value?.port || '');
  return source ? `After ${source.data.label}${branch ? ' · ' + branch : ''}` : '';
});
function chooseNext(node: string, port: string) {
  if (props.readonly) return;
  addFrom.value = { node, port };
  addDialog.value?.showModal();
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
  nodes.value = nodes.value.map((node) =>
    node.id === selectedID.value
      ? { ...node, data: { ...node.data, config: { ...node.data.config, ...patch } } }
      : node,
  );
}
const nodeLabel = computed({
  get: () => selected.value?.data.label || '',
  set: (label: string) => {
    if (!props.readonly)
      nodes.value = nodes.value.map((node) =>
        node.id === selectedID.value ? { ...node, data: { ...node.data, label } } : node,
      );
  },
});
function configField<K extends keyof Config>(key: K) {
  return computed({
    get: () => selected.value?.data.config[key],
    set: (value: Config[K]) => patchConfig({ [key]: value }),
  });
}
const eventName = configField('event_name');
// Email and SMS steps a customer passes before the selected step: the messages a "Wait for
// event" step can wait on.
const earlierMessages = computed(() => {
  const id = selectedID.value;
  if (!id) return [];
  const seen = new Set<string>();
  const queue = [id];
  while (queue.length) {
    const target = queue.shift()!;
    for (const edge of edges.value)
      if (edge.target === target && !seen.has(edge.source)) {
        seen.add(edge.source);
        queue.push(edge.source);
      }
  }
  return nodes.value.filter(
    (n) => seen.has(n.id) && (n.data.kind === 'email' || n.data.kind === 'sms'),
  );
});
const listenFor = computed({
  get: () => (selected.value?.data.config.message_node ? 'message' : 'event'),
  set: (mode: string) => {
    const message = earlierMessages.value[0];
    if (mode === 'message' && message)
      patchConfig({ message_node: message.id, event_name: 'message_clicked' });
    else
      patchConfig({
        message_node: undefined,
        event_name: engagementEvent(eventName.value) ? '' : eventName.value,
      });
  },
});
const messageNode = computed({
  get: () => selected.value?.data.config.message_node,
  set: (id: unknown) => {
    const kind = nodes.value.find((n) => n.id === id)?.data.kind as 'email' | 'sms' | undefined;
    const current = engagementEvent(eventName.value);
    // An SMS has no open or unsubscribe report, so those fall back to a click.
    patchConfig({
      message_node: id as string,
      event_name:
        kind && current && !current.channels.includes(kind) ? 'message_clicked' : eventName.value,
    });
  },
});
const messageKind = computed(
  () =>
    nodes.value.find((n) => n.id === messageNode.value)?.data.kind as 'email' | 'sms' | undefined,
);
const timezone = configField('timezone');
const nodeRule = configField('rule');
const nodeFilters = configField('filters');
const subject = configField('subject');
const preheader = configField('preheader');
const html = configField('html');
const text = configField('text');
const attribute = configField('attribute');
const operation = configField('operation');
const reason = configField('reason');
const percent = configField('percent');
const webhookURL = configField('url');
// Fields are edited as rows so a half-typed name does not lose its value.
const webhookFields = computed(() =>
  Object.entries(selected.value?.data.config.fields || {}).map(([name, path]) => ({ name, path })),
);
function setWebhookFields(rows: { name: string; path: string }[]) {
  patchConfig({ fields: Object.fromEntries(rows.map((row) => [row.name, row.path])) });
}
function updateWebhookField(index: number, patch: { name?: string; path?: string }) {
  setWebhookFields(
    webhookFields.value.map((row, position) => (position === index ? { ...row, ...patch } : row)),
  );
}
function addWebhookField() {
  const rows = webhookFields.value;
  let n = rows.length + 1;
  while (rows.some((row) => row.name === `field_${n}`)) n++;
  setWebhookFields([...rows, { name: `field_${n}`, path: 'customer.attributes.' }]);
}
function removeWebhookField(index: number) {
  setWebhookFields(webhookFields.value.filter((_, position) => position !== index));
}
// Branch outputs are positional, so adding or removing a branch renumbers the connections.
function setBranches(branches: NonNullable<Config['branches']>) {
  const id = selectedID.value;
  patchConfig({ branches });
  // The card gains or loses output handles; have the canvas measure them again.
  void nextTick(() => updateNodeInternals([id]));
}
function addBranch() {
  const branches = selected.value?.data.config.branches || [];
  if (branches.length >= 5) return;
  setBranches([
    ...branches,
    {
      label: `Branch ${branches.length + 1}`,
      rule: { path: 'customer.attributes.tier', operator: 'equals', value: '' },
    },
  ]);
}
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
  setBranches(branches.filter((_, position) => position !== index));
}
function updateBranch(index: number, patch: { label?: string; rule?: Config['rule'] }) {
  setBranches(
    (selected.value?.data.config.branches || []).map((branch, position) =>
      position === index ? { ...branch, ...patch } : branch,
    ),
  );
}
const atTime = configField('at_time');
const datePath = configField('date_path');
const weekdayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const waitMode = computed({
  get: () => selected.value?.data.config.wait_mode || 'duration',
  // Each wait type keeps only its own fields, so a saved step never carries stale ones.
  set: (mode: 'duration' | 'time' | 'date') => {
    const config = selected.value?.data.config;
    if (!config) return;
    patchConfig({
      wait_mode: mode === 'duration' ? undefined : mode,
      duration_seconds: mode === 'duration' ? config.duration_seconds || 7200 : undefined,
      at_time: mode === 'time' ? config.at_time || '09:00' : undefined,
      weekdays: mode === 'time' ? config.weekdays : undefined,
      date_path: mode === 'date' ? config.date_path || 'customer.attributes.renews_on' : undefined,
      offset_seconds: mode === 'date' ? config.offset_seconds || 0 : undefined,
    });
  },
});
// The trigger enrolls on an event or a scheduled audience; each keeps only its own fields.
const lastEventName = ref('');
const entryMode = computed({
  get: () => selected.value?.data.config.entry_mode || 'event',
  set: (mode: 'event' | 'audience') => {
    const config = selected.value?.data.config;
    if (!config || mode === entryMode.value) return;
    if (mode === 'audience') {
      lastEventName.value = config.event_name || '';
      patchConfig({
        entry_mode: 'audience',
        event_name: audienceEvent,
        filters: { path: 'customer.attributes.tier', operator: 'equals', value: 'gold' },
        at_time: '09:00',
        weekdays: undefined,
        timezone: config.timezone || 'UTC',
      });
    } else {
      patchConfig({
        entry_mode: undefined,
        event_name: lastEventName.value || 'cart_updated',
        filters: undefined,
        at_time: undefined,
        weekdays: undefined,
        timezone: undefined,
      });
    }
  },
});
function toggleWeekday(day: number) {
  const days = new Set(selected.value?.data.config.weekdays || []);
  if (days.has(day)) days.delete(day);
  else days.add(day);
  patchConfig({ weekdays: days.size ? [...days].sort((a, b) => a - b) : undefined });
}
// The date offset is edited as an amount, a unit and a direction.
const offsetUnit = ref(86400);
const offsetAmount = computed({
  get: () => Math.abs(selected.value?.data.config.offset_seconds || 0) / offsetUnit.value,
  set: (value: number) =>
    patchConfig({
      offset_seconds: Math.round(Math.abs(Number(value)) * offsetUnit.value) * offsetSign.value,
    }),
});
const offsetSign = computed({
  get: () => ((selected.value?.data.config.offset_seconds || 0) > 0 ? 1 : -1),
  set: (sign: number) =>
    patchConfig({
      offset_seconds: Math.abs(selected.value?.data.config.offset_seconds || 0) * sign,
    }),
});
function changeOffsetUnit(value: number) {
  const amount = offsetAmount.value;
  offsetUnit.value = value;
  offsetAmount.value = amount;
}

const delayUnit = ref(3600);
const delayKey = computed(() =>
  selected.value?.data.kind === 'wait_event' ? 'timeout_seconds' : 'duration_seconds',
);
const delayAmount = computed({
  get: () => (selected.value?.data.config[delayKey.value] || 0) / delayUnit.value,
  set: (value: number) => {
    patchConfig({ [delayKey.value]: Math.round(Number(value) * delayUnit.value) });
  },
});
watch(selectedID, () => {
  inspectorOpen.value = true;
  const seconds = selected.value?.data.config[delayKey.value] || 0;
  delayUnit.value = [86400, 3600, 60].find((unit) => seconds > 0 && seconds % unit === 0) || 1;
  const offset = Math.abs(selected.value?.data.config.offset_seconds || 0);
  offsetUnit.value = [86400, 3600, 60].find((unit) => offset > 0 && offset % unit === 0) || 86400;
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
function drag(event: DragEvent, kind: Kind) {
  event.dataTransfer?.setData('application/journey-node', kind);
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
function setAttribute(value: string) {
  try {
    patchConfig({ value: JSON.parse(value) });
    configError.value = '';
  } catch {
    configError.value = 'Attribute value must be valid JSON';
  }
}
</script>
<template>
  <dialog ref="insertionDialog" class="insert-step-dialog" aria-labelledby="insert-step-title">
    <form @submit.prevent="insertStep">
      <div class="surface-heading">
        <h2 id="insert-step-title">Insert a step</h2>
        <button
          type="button"
          class="icon-button"
          aria-label="Close insertion"
          @click="insertionDialog?.close()"
        >
          <AppIcon name="close" />
        </button>
      </div>
      <p>
        The original branch stays connected to the new step. Choose which output continues to the
        existing next step.
      </p>
      <label
        >Step type<AppSelect
          v-model="insertKind"
          :options="
            insertKinds.map((kind) => ({
              value: kind,
              label: labels[kind],
              description: descriptions[kind],
              icon: iconFor[kind],
              group: stepGroup[kind],
            }))
          "
      /></label>
      <label
        >Continue through<AppSelect
          v-model="insertionPort"
          placeholder="Select an output"
          :options="
            ports(insertKind).map((port) => ({ value: port, label: branchLabel(port) || port }))
          "
      /></label>
      <p v-if="ports(insertKind).length > 1" class="inline-alert">
        Connect the other output before publishing. It is left open for your decision.
      </p>
      <div class="toolbar">
        <button type="button" @click="insertionDialog?.close()">Cancel</button
        ><button class="primary" :disabled="!insertionPort">Insert step</button>
      </div>
    </form>
  </dialog>
  <dialog ref="addDialog" class="add-step-dialog" aria-labelledby="add-step-title">
    <div class="surface-heading">
      <div>
        <h2 id="add-step-title">Add a step</h2>
        <p>{{ addTitle }}</p>
      </div>
      <button
        type="button"
        class="icon-button"
        aria-label="Close step picker"
        @click="addDialog?.close()"
      >
        <AppIcon name="close" />
      </button>
    </div>
    <div class="add-step-options">
      <button v-for="kind in addKinds" :key="kind" type="button" @click="addNext(kind)">
        <span class="step-type-icon" :class="kind"
          ><AppIcon :name="iconFor[kind]" :size="17"
        /></span>
        <span
          ><strong>{{ labels[kind] }}</strong
          ><small>{{ descriptions[kind] }}</small></span
        >
      </button>
    </div>
  </dialog>
  <ShortcutsDialog v-model="shortcutsOpen" />
  <div class="builder-layout">
    <aside v-show="libraryOpen" class="step-library" aria-label="Step library">
      <div class="step-library-heading">
        <h2>Elements</h2>
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
      <div
        class="canvas-bar"
        :class="{ 'beside-ai': aiOpen }"
        role="toolbar"
        aria-label="Canvas controls"
      >
        <button
          type="button"
          class="bar-label"
          aria-label="Toggle step library"
          :aria-expanded="libraryOpen"
          @click="libraryOpen = !libraryOpen"
        >
          <AppIcon name="templates" :size="16" /><span>Steps</span>
        </button>
        <button
          v-if="!readonly"
          type="button"
          class="bar-label bar-ai"
          aria-label="AI composer"
          :aria-pressed="!!aiOpen"
          @click="emit('ai')"
        >
          <AppIcon name="sparkles" :size="16" /><span>AI composer</span>
        </button>
        <i aria-hidden="true" />
        <button
          v-for="[title, icon, handler] in [
            ['Undo', 'undo', undo],
            ['Redo', 'redo', redo],
          ] as const"
          :key="title"
          type="button"
          :disabled="readonly"
          :aria-label="title"
          :title="title"
          @click="handler"
        >
          <AppIcon :name="icon" :size="18" />
        </button>
        <i aria-hidden="true" />
        <button
          type="button"
          class="bar-zoom"
          aria-label="Zoom out"
          title="Zoom out"
          @click="zoomOut()"
        >
          <AppIcon name="minus" :size="18" /></button
        ><span class="canvas-zoom-level bar-zoom" aria-label="Zoom level">{{ zoomLevel }}%</span
        ><button
          type="button"
          class="bar-zoom"
          aria-label="Zoom in"
          title="Zoom in"
          @click="zoomIn()"
        >
          <AppIcon name="plus" :size="18" /></button
        ><button
          type="button"
          aria-label="Fit view"
          title="Fit view"
          @click="
            fitView({ padding: { top: '56px', right: '40px', bottom: '104px', left: '40px' } })
          "
        >
          <AppIcon name="maximize" :size="18" />
        </button>
        <i aria-hidden="true" />
        <button
          type="button"
          aria-label="Toggle step settings"
          title="Step settings"
          :aria-expanded="!!selected && inspectorOpen && !hideInspector"
          :disabled="!selected"
          @click="inspectorOpen = !inspectorOpen"
        >
          <AppIcon name="settings" :size="18" />
        </button>
        <details ref="moreMenu" class="bar-more">
          <summary aria-label="More canvas actions" title="More">
            <AppIcon name="more" :size="18" />
          </summary>
          <div>
            <label
              v-if="sharedExits.size"
              class="menu-check"
              title="Shared exits appear as references. Select one to open the original step."
              ><input v-model="shortExitLinks" type="checkbox" />Short exit links</label
            >
            <button type="button" :disabled="readonly" @click="fromMenu(layout)">
              Auto layout
            </button>
            <button type="button" @click="fromMenu(startView)">Go to start</button>
            <button
              type="button"
              :disabled="!selectedID"
              @click="fromMenu(() => focusNode(selectedID))"
            >
              Focus selected step
            </button>
            <hr />
            <button type="button" :disabled="readonly" @click="fromMenu(selectAll)">
              Select all nodes
            </button>
            <button type="button" :disabled="readonly" @click="fromMenu(copy)">
              Copy selection
            </button>
            <button type="button" :disabled="readonly" @click="fromMenu(paste)">
              Paste selection
            </button>
            <button type="button" :disabled="readonly" @click="fromMenu(duplicate)">
              Duplicate node
            </button>
            <hr />
            <button v-if="!readonly" type="button" @click="fromMenu(() => (locked = !locked))">
              {{ locked ? 'Unlock canvas' : 'Lock canvas' }}
            </button>
            <button type="button" @click="fromMenu(() => (shortcutsOpen = true))">
              Keyboard shortcuts
            </button>
          </div>
        </details>
      </div>
    </div>
    <aside
      v-if="selected && !hideInspector && inspectorOpen"
      class="node-config"
      aria-label="Step settings"
    >
      <div class="inspector-heading">
        <span class="step-type-icon" :class="selected.data.kind"
          ><AppIcon :name="iconFor[selected.data.kind]" :size="18"
        /></span>
        <div>
          <h2>{{ labels[selected.data.kind] }}</h2>
        </div>
        <button class="icon-button" aria-label="Close step settings" @click="selectedID = ''">
          <AppIcon name="close" :size="17" />
        </button>
      </div>
      <p class="inspector-help">{{ explanations[selected.data.kind] }}</p>
      <div
        v-if="issues.some((i) => i.node_id === selectedID)"
        class="inspector-issues"
        role="alert"
      >
        <strong>Needs attention before publishing</strong>
        <p
          v-for="issue in issues.filter((i) => i.node_id === selectedID)"
          :key="issue.field + issue.message"
        >
          {{ issue.message }}
        </p>
      </div>
      <section class="inspector-group">
        <label>Step name<input v-model="nodeLabel" :disabled="readonly" /></label>
      </section>
      <section class="inspector-group">
        <h4>{{ groupTitle[selected.data.kind] }}</h4>
        <SegmentedControl
          v-if="selected.data.kind === 'event_trigger'"
          v-model="entryMode"
          label="Entry type"
          :disabled="readonly"
          :options="[
            { value: 'event', label: 'An event', icon: 'zap' },
            { value: 'audience', label: 'A scheduled audience', icon: 'users' },
          ]"
        />
        <template v-if="selected.data.kind === 'event_trigger' && entryMode === 'audience'">
          <div class="audience-rule">
            <span class="field-title">Who enters</span>
            <RuleEditor v-model="nodeFilters" :readonly="readonly" />
          </div>
          <label
            >Time of day<input v-model="atTime" type="time" required :disabled="readonly"
          /></label>
          <fieldset class="weekday-picker">
            <legend>Run on these days</legend>
            <button
              v-for="(day, index) in weekdayNames"
              :key="day"
              type="button"
              :aria-pressed="(selected.data.config.weekdays || []).includes(index + 1)"
              :disabled="readonly"
              @click="toggleWeekday(index + 1)"
            >
              {{ day }}
            </button>
            <small>Leave all off to run every day.</small>
          </fieldset>
          <label>Timezone<input v-model="timezone" :disabled="readonly" /></label>
          <small
            >Customers who match at that time enter together. Re-entry settings decide whether they
            can enter again on a later run.</small
          >
        </template>
        <template v-else-if="selected.data.kind === 'wait_event'">
          <SegmentedControl
            v-if="earlierMessages.length || listenFor === 'message'"
            v-model="listenFor"
            label="Listen for"
            :disabled="readonly"
            :options="[
              { value: 'event', label: 'A customer event' },
              { value: 'message', label: 'Message engagement' },
            ]"
          />
          <template v-if="listenFor === 'message'">
            <label
              >Message<AppSelect
                v-model="messageNode"
                :disabled="readonly"
                placeholder="Choose an earlier message"
                :options="
                  earlierMessages.map((n) => ({
                    value: n.id,
                    label: n.data.label,
                    icon: iconFor[n.data.kind],
                  }))
                "
            /></label>
            <label
              >The customer<AppSelect
                v-model="eventName"
                :disabled="readonly || !messageKind"
                :options="
                  engagementEvents
                    .filter((e) => !messageKind || e.channels.includes(messageKind))
                    .map((e) => ({ value: e.value, label: e.label, description: e.description }))
                "
            /></label>
            <small
              >Only this customer's copy of that message counts. Links are tracked and an
              unsubscribe link is added to emails when they are sent.</small
            >
          </template>
          <label v-else>Event name<input v-model="eventName" :disabled="readonly" /></label>
        </template>
        <label v-else-if="selected.data.kind === 'event_trigger'"
          >Event name<input v-model="eventName" :disabled="readonly"
        /></label>
        <SegmentedControl
          v-if="selected.data.kind === 'wait'"
          v-model="waitMode"
          label="Wait type"
          :disabled="readonly"
          :options="[
            { value: 'duration', label: 'Duration' },
            { value: 'time', label: 'Time of day' },
            { value: 'date', label: 'Customer date' },
          ]"
        />
        <template v-if="selected.data.kind === 'wait' && waitMode === 'time'">
          <label
            >Time of day<input v-model="atTime" type="time" required :disabled="readonly"
          /></label>
          <fieldset class="weekday-picker">
            <legend>Only on these days</legend>
            <button
              v-for="(day, index) in weekdayNames"
              :key="day"
              type="button"
              :aria-pressed="(selected.data.config.weekdays || []).includes(index + 1)"
              :disabled="readonly"
              @click="toggleWeekday(index + 1)"
            >
              {{ day }}
            </button>
            <small>Leave all off to continue on any day.</small>
          </fieldset>
        </template>
        <template v-if="selected.data.kind === 'wait' && waitMode === 'date'">
          <label
            >Date field<input
              v-model="datePath"
              :disabled="readonly"
              placeholder="customer.attributes.renews_on"
          /></label>
          <div class="duration-field offset-field">
            <label
              >Offset<input
                v-model.number="offsetAmount"
                type="number"
                min="0"
                :disabled="readonly"
            /></label>
            <label
              >Offset unit<AppSelect
                :model-value="offsetUnit"
                aria-label="Offset unit"
                :disabled="readonly"
                :options="[
                  { value: 60, label: 'Minutes' },
                  { value: 3600, label: 'Hours' },
                  { value: 86400, label: 'Days' },
                ]"
                @update:model-value="changeOffsetUnit(Number($event))"
            /></label>
            <SegmentedControl
              v-model="offsetSign"
              label="Direction"
              :disabled="readonly"
              :options="[
                { value: -1, label: 'Before' },
                { value: 1, label: 'After' },
              ]"
            />
          </div>
          <small
            >A customer without this date, or whose date has passed, continues straight away.</small
          >
        </template>
        <div
          v-if="
            selected.data.kind === 'wait_event' ||
            (selected.data.kind === 'wait' && waitMode === 'duration')
          "
          class="duration-field"
        >
          <label
            >{{ selected.data.kind === 'wait_event' ? 'Timeout' : 'Wait for'
            }}<input v-model.number="delayAmount" type="number" min="1" :disabled="readonly"
          /></label>
          <label
            >Unit<AppSelect
              :model-value="delayUnit"
              aria-label="Delay unit"
              :disabled="readonly"
              :options="[
                { value: 1, label: 'Seconds' },
                { value: 60, label: 'Minutes' },
                { value: 3600, label: 'Hours' },
                { value: 86400, label: 'Days' },
              ]"
              @update:model-value="changeDelayUnit(Number($event))"
          /></label>
        </div>
        <label v-if="selected.data.kind === 'wait'"
          >Timezone<input v-model="timezone" :disabled="readonly"
        /></label>
        <template v-if="selected.data.kind === 'switch'">
          <div
            v-for="(branch, index) in selected.data.config.branches || []"
            :key="index"
            class="branch-editor"
          >
            <div class="branch-editor-heading">
              <span>{{ index + 1 }}</span>
              <input
                :value="branch.label"
                :aria-label="`Name of branch ${index + 1}`"
                maxlength="60"
                :disabled="readonly"
                @input="updateBranch(index, { label: ($event.target as HTMLInputElement).value })"
              />
              <button
                v-if="!readonly && (selected.data.config.branches || []).length > 2"
                type="button"
                class="icon-button"
                :aria-label="`Remove branch ${index + 1}`"
                @click="removeBranch(index)"
              >
                <AppIcon name="close" :size="14" />
              </button>
            </div>
            <RuleEditor
              :model-value="branch.rule"
              :readonly="readonly"
              @update:model-value="updateBranch(index, { rule: $event })"
            />
          </div>
          <button
            v-if="!readonly && (selected.data.config.branches || []).length < 5"
            type="button"
            @click="addBranch"
          >
            <AppIcon name="plus" :size="14" />Add a branch
          </button>
          <small>The first branch that matches wins. Everyone else takes Otherwise.</small>
        </template>
        <template v-if="selected.data.kind === 'split'">
          <label
            >Share for variant A (%)<input
              v-model.number="percent"
              type="range"
              min="1"
              max="99"
              :disabled="readonly"
          /></label>
          <div class="split-share" aria-live="polite">
            <span class="variant-a" :style="{ flexGrow: percent || 0 }">A {{ percent || 0 }}%</span
            ><span class="variant-b" :style="{ flexGrow: 100 - (percent || 0) }"
              >B {{ 100 - (percent || 0) }}%</span
            >
          </div>
          <small>Compare the variants in Analytics once customers have passed through both.</small>
        </template>
        <template v-if="selected.data.kind === 'webhook'">
          <label
            >Address<input
              v-model.trim="webhookURL"
              type="url"
              inputmode="url"
              placeholder="https://"
              :disabled="readonly"
          /></label>
          <fieldset class="webhook-fields">
            <legend>Fields sent</legend>
            <div v-for="(row, index) in webhookFields" :key="index" class="webhook-field">
              <input
                :value="row.name"
                :aria-label="`Name of field ${index + 1}`"
                :disabled="readonly"
                @change="
                  updateWebhookField(index, {
                    name: ($event.target as HTMLInputElement).value.trim(),
                  })
                "
              />
              <input
                :value="row.path"
                :aria-label="`Value of field ${index + 1}`"
                placeholder="customer.attributes.plan"
                :disabled="readonly"
                @change="
                  updateWebhookField(index, {
                    path: ($event.target as HTMLInputElement).value.trim(),
                  })
                "
              />
              <button
                v-if="!readonly"
                type="button"
                class="icon-button"
                :aria-label="`Remove field ${index + 1}`"
                @click="removeWebhookField(index)"
              >
                <AppIcon name="close" :size="14" />
              </button>
            </div>
            <button
              v-if="!readonly && webhookFields.length < 20"
              type="button"
              @click="addWebhookField"
            >
              <AppIcon name="plus" :size="14" />Add a field
            </button>
          </fieldset>
          <small
            >Sent as JSON with the customer and execution IDs. Each call carries an Idempotency-Key
            header, and a missing value is sent as null.</small
          >
        </template>
        <RuleEditor
          v-if="selected.data.kind === 'condition'"
          v-model="nodeRule"
          :readonly="readonly"
        />
        <details
          v-if="
            selected.data.kind === 'wait_event' ||
            (selected.data.kind === 'event_trigger' && entryMode !== 'audience')
          "
        >
          <summary>Event filters</summary>
          <button
            v-if="!selected.data.config.filters"
            :disabled="readonly"
            @click="
              nodeFilters = {
                path: 'event.properties.amount',
                operator: 'greater_than',
                value: 0,
              }
            "
          >
            Add filter</button
          ><RuleEditor v-else v-model="nodeFilters" :readonly="readonly" />
        </details>
        <p v-if="configError" role="alert" class="error-message">{{ configError }}</p>
        <template v-if="selected.data.kind === 'email'"
          ><label>Subject<input v-model="subject" :disabled="readonly" /></label
          ><label>Preheader<input v-model="preheader" :disabled="readonly" /></label
          ><label
            >HTML content<textarea
              v-model="html"
              rows="8"
              class="code-input"
              :disabled="readonly"
            /></label></template
        ><label v-if="['email', 'sms'].includes(selected.data.kind)"
          >Message text<textarea v-model="text" :disabled="readonly" rows="5" /></label
        ><template v-if="selected.data.kind === 'update_attribute'"
          ><label>Attribute<input v-model="attribute" :disabled="readonly" /></label
          ><SegmentedControl
            v-model="operation"
            label="Operation"
            :disabled="readonly"
            :options="[
              { value: 'set', label: 'Set' },
              { value: 'increment', label: 'Add to' },
              { value: 'unset', label: 'Remove' },
            ]" /><label
            >Value (JSON)<input
              :value="JSON.stringify(selected.data.config.value)"
              :disabled="readonly"
              @change="setAttribute(($event.target as HTMLInputElement).value)" /></label></template
        ><label v-if="selected.data.kind === 'exit'"
          >Exit reason<AppSelect
            v-model="reason"
            :disabled="readonly"
            :options="[
              {
                value: 'converted',
                label: 'Converted',
                description: 'The goal was reached',
                tone: 'green',
              },
              {
                value: 'completed',
                label: 'Completed',
                description: 'Finished without converting',
                tone: 'blue',
              },
              {
                value: 'condition_not_met',
                label: 'Not eligible',
                description: 'No longer meets the entry rules',
                tone: 'amber',
              },
              {
                value: 'manually_cancelled',
                label: 'Cancelled',
                description: 'Stopped by an operator',
                tone: 'neutral',
              },
            ]"
        /></label>
        <button
          v-if="['email', 'sms'].includes(selected.data.kind)"
          type="button"
          @click="emit('preview', selected.id)"
        >
          <AppIcon name="play" :size="14" />Preview and test this message
        </button>
      </section>
      <section class="inspector-group">
        <h4>Connections</h4>
        <ul class="inspector-links">
          <li v-for="link in incomingSteps" :key="link.id">
            <span>From</span>
            <button type="button" :title="'Go to ' + link.label" @click="focusNode(link.source)">
              {{ link.label }}<template v-if="link.branch"> · {{ link.branch }}</template>
            </button>
          </li>
          <li v-if="!incomingSteps.length && selected.data.kind !== 'event_trigger'">
            <span>From</span><em>Nothing leads here yet</em>
          </li>
          <li v-for="link in outgoingSteps" :key="link.port">
            <span>{{ link.name }}</span>
            <button
              v-if="link.target"
              type="button"
              :title="'Go to ' + link.label"
              @click="focusNode(link.target)"
            >
              {{ link.label }}
            </button>
            <button
              v-else-if="!readonly"
              type="button"
              class="inspector-add"
              @click="chooseNext(selected.id, link.port)"
            >
              <AppIcon name="plus" :size="13" />Add a step
            </button>
            <em v-else>Not connected</em>
          </li>
          <li v-if="selected.data.kind === 'exit'"><span>Then</span><em>The journey ends</em></li>
        </ul>
      </section>
      <footer v-if="!readonly" class="inspector-actions">
        <button type="button" class="inspector-delete" @click="remove">Delete step</button>
        <small>Undo brings it back.</small>
      </footer>
    </aside>
  </div>
</template>
