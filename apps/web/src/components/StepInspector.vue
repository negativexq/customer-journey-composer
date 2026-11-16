<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import AppIcon from './ui/AppIcon.vue';
import RuleEditor from './RuleEditor.vue';
import { audienceEvent, labels, ports, type Config, type Issue } from '../lib/workflow';
import { portLabel } from '../lib/workflow-presentation';
import { engagementEvent, engagementEvents } from '../lib/engagement';
import {
  branchLabel,
  explanations,
  groupTitle,
  iconFor,
  type CanvasEdge,
  type CanvasNode,
} from '../lib/step-catalog';

// The settings of the step selected on the canvas. Every change goes back to the canvas as a
// patch, so the canvas stays the one owner of the journey and its undo history.
const props = defineProps<{
  step: CanvasNode;
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  issues: Issue[];
  readonly?: boolean;
  error: string;
}>();
const emit = defineEmits<{
  patch: [Partial<Config>];
  rename: [string];
  'remove-branch': [number];
  error: [string];
  close: [];
  focus: [string];
  'add-next': [string, string];
  remove: [];
  preview: [string];
}>();
// The event a trigger listened for before it became an audience; the canvas keeps it so it
// survives the settings panel closing.
const lastEventName = defineModel<string>('lastEventName', { default: '' });

const config = computed(() => props.step.data.config);
function patchConfig(patch: Partial<Config>) {
  if (!props.readonly) emit('patch', patch);
}
const nodeLabels = computed(() => new Map(props.nodes.map((node) => [node.id, node.data.label])));
const stepIssues = computed(() => props.issues.filter((issue) => issue.node_id === props.step.id));

// What leads into the step and where each of its outputs goes.
const incomingSteps = computed(() =>
  props.edges
    .filter((edge) => edge.target === props.step.id)
    .map((edge) => ({
      id: edge.id,
      source: edge.source,
      label: nodeLabels.value.get(edge.source) || edge.source,
      branch: branchLabel(edge.sourceHandle || ''),
    })),
);
const outgoingSteps = computed(() =>
  ports(props.step.data.kind, config.value).map((port) => {
    const edge = props.edges.find(
      (candidate) =>
        candidate.source === props.step.id && (candidate.sourceHandle || 'next') === port,
    );
    return {
      port,
      name: portLabel(port),
      target: edge?.target,
      label: edge ? nodeLabels.value.get(edge.target) || edge.target : '',
    };
  }),
);

const nodeLabel = computed({
  get: () => props.step.data.label || '',
  set: (label: string) => {
    if (!props.readonly) emit('rename', label);
  },
});
function configField<K extends keyof Config>(key: K) {
  return computed({
    get: () => config.value[key],
    set: (value: Config[K]) => patchConfig({ [key]: value }),
  });
}
const eventName = configField('event_name');
// Email and SMS steps a customer passes before this step: the messages a "Wait for event"
// step can wait on.
const earlierMessages = computed(() => {
  const seen = new Set<string>();
  const queue = [props.step.id];
  while (queue.length) {
    const target = queue.shift()!;
    for (const edge of props.edges)
      if (edge.target === target && !seen.has(edge.source)) {
        seen.add(edge.source);
        queue.push(edge.source);
      }
  }
  return props.nodes.filter(
    (n) => seen.has(n.id) && (n.data.kind === 'email' || n.data.kind === 'sms'),
  );
});
const listenFor = computed({
  get: () => (config.value.message_node ? 'message' : 'event'),
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
  get: () => config.value.message_node,
  set: (id: unknown) => {
    const kind = props.nodes.find((n) => n.id === id)?.data.kind as 'email' | 'sms' | undefined;
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
    props.nodes.find((n) => n.id === messageNode.value)?.data.kind as 'email' | 'sms' | undefined,
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
  Object.entries(config.value.fields || {}).map(([name, path]) => ({ name, path })),
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
function addBranch() {
  const branches = config.value.branches || [];
  if (branches.length >= 5) return;
  patchConfig({
    branches: [
      ...branches,
      {
        label: `Branch ${branches.length + 1}`,
        rule: { path: 'customer.attributes.tier', operator: 'equals', value: '' },
      },
    ],
  });
}
function updateBranch(index: number, patch: { label?: string; rule?: Config['rule'] }) {
  patchConfig({
    branches: (config.value.branches || []).map((branch, position) =>
      position === index ? { ...branch, ...patch } : branch,
    ),
  });
}
const atTime = configField('at_time');
const datePath = configField('date_path');
const weekdayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const waitMode = computed({
  get: () => config.value.wait_mode || 'duration',
  // Each wait type keeps only its own fields, so a saved step never carries stale ones.
  set: (mode: 'duration' | 'time' | 'date') => {
    const current = config.value;
    patchConfig({
      wait_mode: mode === 'duration' ? undefined : mode,
      duration_seconds: mode === 'duration' ? current.duration_seconds || 7200 : undefined,
      at_time: mode === 'time' ? current.at_time || '09:00' : undefined,
      weekdays: mode === 'time' ? current.weekdays : undefined,
      date_path: mode === 'date' ? current.date_path || 'customer.attributes.renews_on' : undefined,
      offset_seconds: mode === 'date' ? current.offset_seconds || 0 : undefined,
    });
  },
});
// The trigger enrolls on an event or a scheduled audience; each keeps only its own fields.
const entryMode = computed({
  get: () => config.value.entry_mode || 'event',
  set: (mode: 'event' | 'audience') => {
    const current = config.value;
    if (mode === entryMode.value) return;
    if (mode === 'audience') {
      lastEventName.value = current.event_name || '';
      patchConfig({
        entry_mode: 'audience',
        event_name: audienceEvent,
        filters: { path: 'customer.attributes.tier', operator: 'equals', value: 'gold' },
        at_time: '09:00',
        weekdays: undefined,
        timezone: current.timezone || 'UTC',
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
  const days = new Set(config.value.weekdays || []);
  if (days.has(day)) days.delete(day);
  else days.add(day);
  patchConfig({ weekdays: days.size ? [...days].sort((a, b) => a - b) : undefined });
}
// The date offset is edited as an amount, a unit and a direction.
const offsetUnit = ref(86400);
const offsetAmount = computed({
  get: () => Math.abs(config.value.offset_seconds || 0) / offsetUnit.value,
  set: (value: number) =>
    patchConfig({
      offset_seconds: Math.round(Math.abs(Number(value)) * offsetUnit.value) * offsetSign.value,
    }),
});
const offsetSign = computed({
  get: () => ((config.value.offset_seconds || 0) > 0 ? 1 : -1),
  set: (sign: number) =>
    patchConfig({ offset_seconds: Math.abs(config.value.offset_seconds || 0) * sign }),
});
function changeOffsetUnit(value: number) {
  const amount = offsetAmount.value;
  offsetUnit.value = value;
  offsetAmount.value = amount;
}

const delayUnit = ref(3600);
const delayKey = computed(() =>
  props.step.data.kind === 'wait_event' ? 'timeout_seconds' : 'duration_seconds',
);
const delayAmount = computed({
  get: () => (config.value[delayKey.value] || 0) / delayUnit.value,
  set: (value: number) => {
    patchConfig({ [delayKey.value]: Math.round(Number(value) * delayUnit.value) });
  },
});
// Each step opens in the largest unit its saved value divides into.
watch(
  () => props.step.id,
  () => {
    const seconds = config.value[delayKey.value] || 0;
    delayUnit.value = [86400, 3600, 60].find((unit) => seconds > 0 && seconds % unit === 0) || 1;
    const offset = Math.abs(config.value.offset_seconds || 0);
    offsetUnit.value = [86400, 3600, 60].find((unit) => offset > 0 && offset % unit === 0) || 86400;
  },
  { immediate: true },
);
function changeDelayUnit(value: number) {
  const amount = delayAmount.value;
  delayUnit.value = value;
  delayAmount.value = amount;
}
function setAttribute(value: string) {
  try {
    patchConfig({ value: JSON.parse(value) });
    emit('error', '');
  } catch {
    emit('error', 'Attribute value must be valid JSON');
  }
}
</script>
<template>
  <aside class="node-config" aria-label="Step settings">
    <div class="inspector-heading">
      <span class="step-type-icon" :class="step.data.kind"
        ><AppIcon :name="iconFor[step.data.kind]" :size="18"
      /></span>
      <div>
        <h2>{{ labels[step.data.kind] }}</h2>
      </div>
      <button class="icon-button" aria-label="Close step settings" @click="emit('close')">
        <AppIcon name="close" :size="17" />
      </button>
    </div>
    <p class="inspector-help">{{ explanations[step.data.kind] }}</p>
    <div v-if="stepIssues.length" class="inspector-issues" role="alert">
      <strong>Needs attention before publishing</strong>
      <p v-for="issue in stepIssues" :key="issue.field + issue.message">
        {{ issue.message }}
      </p>
    </div>
    <section class="inspector-group">
      <label>Step name<input v-model="nodeLabel" :disabled="readonly" /></label>
    </section>
    <section class="inspector-group">
      <h4>{{ groupTitle[step.data.kind] }}</h4>
      <SegmentedControl
        v-if="step.data.kind === 'event_trigger'"
        v-model="entryMode"
        label="Entry type"
        :disabled="readonly"
        :options="[
          { value: 'event', label: 'An event', icon: 'zap' },
          { value: 'audience', label: 'A scheduled audience', icon: 'users' },
        ]"
      />
      <template v-if="step.data.kind === 'event_trigger' && entryMode === 'audience'">
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
            :aria-pressed="(config.weekdays || []).includes(index + 1)"
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
      <template v-else-if="step.data.kind === 'wait_event'">
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
            >Only this customer's copy of that message counts. Links are tracked and an unsubscribe
            link is added to emails when they are sent.</small
          >
        </template>
        <label v-else>Event name<input v-model="eventName" :disabled="readonly" /></label>
      </template>
      <label v-else-if="step.data.kind === 'event_trigger'"
        >Event name<input v-model="eventName" :disabled="readonly"
      /></label>
      <SegmentedControl
        v-if="step.data.kind === 'wait'"
        v-model="waitMode"
        label="Wait type"
        :disabled="readonly"
        :options="[
          { value: 'duration', label: 'Duration' },
          { value: 'time', label: 'Time of day' },
          { value: 'date', label: 'Customer date' },
        ]"
      />
      <template v-if="step.data.kind === 'wait' && waitMode === 'time'">
        <label
          >Time of day<input v-model="atTime" type="time" required :disabled="readonly"
        /></label>
        <fieldset class="weekday-picker">
          <legend>Only on these days</legend>
          <button
            v-for="(day, index) in weekdayNames"
            :key="day"
            type="button"
            :aria-pressed="(config.weekdays || []).includes(index + 1)"
            :disabled="readonly"
            @click="toggleWeekday(index + 1)"
          >
            {{ day }}
          </button>
          <small>Leave all off to continue on any day.</small>
        </fieldset>
      </template>
      <template v-if="step.data.kind === 'wait' && waitMode === 'date'">
        <label
          >Date field<input
            v-model="datePath"
            :disabled="readonly"
            placeholder="customer.attributes.renews_on"
        /></label>
        <div class="duration-field offset-field">
          <label
            >Offset<input v-model.number="offsetAmount" type="number" min="0" :disabled="readonly"
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
          step.data.kind === 'wait_event' || (step.data.kind === 'wait' && waitMode === 'duration')
        "
        class="duration-field"
      >
        <label
          >{{ step.data.kind === 'wait_event' ? 'Timeout' : 'Wait for'
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
      <label v-if="step.data.kind === 'wait'"
        >Timezone<input v-model="timezone" :disabled="readonly"
      /></label>
      <template v-if="step.data.kind === 'switch'">
        <div v-for="(branch, index) in config.branches || []" :key="index" class="branch-editor">
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
              v-if="!readonly && (config.branches || []).length > 2"
              type="button"
              class="icon-button"
              :aria-label="`Remove branch ${index + 1}`"
              @click="emit('remove-branch', index)"
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
          v-if="!readonly && (config.branches || []).length < 5"
          type="button"
          @click="addBranch"
        >
          <AppIcon name="plus" :size="14" />Add a branch
        </button>
        <small>The first branch that matches wins. Everyone else takes Otherwise.</small>
      </template>
      <template v-if="step.data.kind === 'split'">
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
      <template v-if="step.data.kind === 'webhook'">
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
      <RuleEditor v-if="step.data.kind === 'condition'" v-model="nodeRule" :readonly="readonly" />
      <details
        v-if="
          step.data.kind === 'wait_event' ||
          (step.data.kind === 'event_trigger' && entryMode !== 'audience')
        "
      >
        <summary>Event filters</summary>
        <button
          v-if="!config.filters"
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
      <p v-if="error" role="alert" class="error-message">{{ error }}</p>
      <template v-if="step.data.kind === 'email'"
        ><label>Subject<input v-model="subject" :disabled="readonly" /></label
        ><label>Preheader<input v-model="preheader" :disabled="readonly" /></label
        ><label
          >HTML content<textarea
            v-model="html"
            rows="8"
            class="code-input"
            :disabled="readonly"
          /></label></template
      ><label v-if="['email', 'sms'].includes(step.data.kind)"
        >Message text<textarea v-model="text" :disabled="readonly" rows="5" /></label
      ><template v-if="step.data.kind === 'update_attribute'"
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
            :value="JSON.stringify(config.value)"
            :disabled="readonly"
            @change="setAttribute(($event.target as HTMLInputElement).value)" /></label></template
      ><label v-if="step.data.kind === 'exit'"
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
        v-if="['email', 'sms'].includes(step.data.kind)"
        type="button"
        @click="emit('preview', step.id)"
      >
        <AppIcon name="play" :size="14" />Preview and test this message
      </button>
    </section>
    <section class="inspector-group">
      <h4>Connections</h4>
      <ul class="inspector-links">
        <li v-for="link in incomingSteps" :key="link.id">
          <span>From</span>
          <button type="button" :title="'Go to ' + link.label" @click="emit('focus', link.source)">
            {{ link.label }}<template v-if="link.branch"> · {{ link.branch }}</template>
          </button>
        </li>
        <li v-if="!incomingSteps.length && step.data.kind !== 'event_trigger'">
          <span>From</span><em>Nothing leads here yet</em>
        </li>
        <li v-for="link in outgoingSteps" :key="link.port">
          <span>{{ link.name }}</span>
          <button
            v-if="link.target"
            type="button"
            :title="'Go to ' + link.label"
            @click="emit('focus', link.target)"
          >
            {{ link.label }}
          </button>
          <button
            v-else-if="!readonly"
            type="button"
            class="inspector-add"
            @click="emit('add-next', step.id, link.port)"
          >
            <AppIcon name="plus" :size="13" />Add a step
          </button>
          <em v-else>Not connected</em>
        </li>
        <li v-if="step.data.kind === 'exit'"><span>Then</span><em>The journey ends</em></li>
      </ul>
    </section>
    <footer v-if="!readonly" class="inspector-actions">
      <button type="button" class="inspector-delete" @click="emit('remove')">Delete step</button>
      <small>Undo brings it back.</small>
    </footer>
  </aside>
</template>
