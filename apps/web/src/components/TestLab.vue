<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import type { TestRunSummary } from '../lib/test-runs';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { errorMessage } from '../lib/format';
import { computed, onActivated, onMounted, ref, watch } from 'vue';
import { api, type Journey } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import CustomerProfileFields from './ui/CustomerProfileFields.vue';
import TestInbox from './TestInbox.vue';
import VirtualClock from './VirtualClock.vue';
import SandboxResult from './SandboxResult.vue';
import RecentTestRuns from './RecentTestRuns.vue';
import type { RunState, SandboxRun } from '../lib/test-lab';
import ScenarioFields from './ui/ScenarioFields.vue';
import { testContext, type TestContext, type ScenarioDraft } from '../lib/test-context';
import type { Workflow } from '../lib/workflow';
const props = defineProps<{
  journeyId?: string;
  workflow?: Workflow;
  journeyTags?: string[];
  beforeRun?: () => Promise<void>;
  readonly?: boolean;
  nodeId?: string;
}>();
type Example = {
  name: string;
  start_time: string;
  customer: Record<string, unknown>;
  entry_event: Record<string, unknown>;
  events?: unknown[];
  attribute_changes?: unknown[];
  assertions: unknown[];
};
const examples = ref<Example[]>([]);
const selectedExample = ref('');
const exampleNotes = ref<string[]>([]);
let exampleRequest = 0;
const context = ref<TestContext>();
const sourceLoading = ref(false);
const sourceFailed = ref(false);
const sourceChanged = ref(false);
const intentionalEntry = ref(false);
const selectedSuggestion = ref('');
const reviewedSuggestion = ref(false);
let lastSource = '',
  defaultInputs = '';
let applyingInputs = false;
const formSignature = () =>
  JSON.stringify([
    name.value,
    customer.value,
    entry.value,
    events.value,
    changes.value,
    assertions.value,
  ]);
const mismatchedEntry = computed(
  () =>
    !savedScenario.value &&
    mode.value === 'full' &&
    !!context.value?.entryName &&
    entryName.value !== context.value.entryName,
);
// Why Create test run is unavailable, said next to the button.
const createBlocker = computed(() => {
  if (busy.value) return '';
  if (props.readonly) return 'Your role can view test runs but not create them.';
  if (!journey.value && !savedScenario.value) return 'Choose a journey or a saved scenario first.';
  if (sourceFailed.value && !savedScenario.value)
    return 'The journey could not be loaded; choose a saved scenario or try again.';
  if (mismatchedEntry.value && !intentionalEntry.value)
    return 'The entry event does not start this journey. Change it or mark it as intentional.';
  return '';
});
const customerFields = computed(() =>
  (context.value?.fields || []).filter(
    (field) =>
      field.path.startsWith('customer.') &&
      ![
        'customer.email',
        'customer.phone',
        'customer.timezone',
        'customer.attributes.first_name',
        'customer.consents.email',
        'customer.consents.sms',
        'customer.consents.push',
      ].includes(field.path),
  ),
);
const eventFields = computed(() =>
  (context.value?.fields || []).filter((field) => field.path.startsWith('event.properties.')),
);
async function loadExamples(autofill = true) {
  const request = ++exampleRequest;
  if (!journey.value) {
    context.value = undefined;
    examples.value = [];
    return;
  }
  sourceLoading.value = true;
  sourceFailed.value = false;
  try {
    const selected =
      props.workflow && props.journeyId === journey.value
        ? { tags: props.journeyTags || [], draft: props.workflow }
        : await api<Journey>('/journeys/' + journey.value);
    const workflow =
      version.value > 0
        ? (
            await api<{ workflow: Workflow }>(
              `/journeys/${journey.value}/versions/${version.value}`,
            )
          ).workflow
        : (selected.draft as unknown as Workflow);
    if (request !== exampleRequest) return;
    const key = JSON.stringify([journey.value, version.value, workflow]);
    const changed = !!lastSource && key !== lastSource;
    const pristine = !defaultInputs || formSignature() === defaultInputs;
    context.value = testContext(workflow);
    if (key === lastSource) return;
    sourceChanged.value = changed;
    examples.value = [];
    exampleNotes.value = [];
    if (
      selected.tags.includes('eur-recovery') &&
      workflow.acceptance_contract === 'cart-recovery-eur.v1'
    ) {
      const data = await api<{ scenarios: Example[]; notes: string[] }>(
        '/templates/abandoned-cart-eur',
      );
      if (request !== exampleRequest) return;
      examples.value = data.scenarios;
      exampleNotes.value = data.notes;
      if (autofill && pristine && !savedScenario.value && !props.nodeId && version.value === 0) {
        selectedExample.value = '2';
        applyExample();
      }
    } else if (autofill && pristine && !savedScenario.value) applyDefaults();
    lastSource = key;
  } catch (cause) {
    if (request === exampleRequest) {
      sourceFailed.value = true;
      error.value = errorMessage(cause);
    }
  } finally {
    if (request === exampleRequest) sourceLoading.value = false;
  }
}
function applyScenario(scene: ScenarioDraft) {
  applyingInputs = true;
  savedScenario.value = '';
  selectedExample.value = '';
  customerID.value = '';
  name.value = scene.name;
  start.value = scene.start_time.slice(0, 19);
  mode.value = props.nodeId ? 'single_node' : 'full';
  node.value = props.nodeId || '';
  customer.value = JSON.stringify(scene.customer, null, 2);
  entry.value = JSON.stringify(scene.entry_event, null, 2);
  events.value = JSON.stringify(scene.events, null, 2);
  changes.value = JSON.stringify(scene.attribute_changes, null, 2);
  outcome.value = 'any';
  expectedMessages.value = 'any';
  assertions.value = JSON.stringify(
    scene.assertions.filter((check) => mode.value === 'full' || check.type !== 'enrollment_count'),
    null,
    2,
  );
  faults.value = '[]';
  prior.value = 0;
  exploratory.value = true;
  intentionalEntry.value = false;
  run.value = undefined;
  path.value = undefined;
  sourceChanged.value = false;
  applyingInputs = false;
  defaultInputs = formSignature();
}
function applyDefaults() {
  if (context.value) {
    selectedSuggestion.value = '';
    reviewedSuggestion.value = false;
    applyScenario(context.value.sample);
  }
}
function applySuggestion() {
  const suggestion = context.value?.suggestions[Number(selectedSuggestion.value)];
  if (!suggestion || selectedSuggestion.value === '') return;
  reviewedSuggestion.value = false;
  applyScenario(suggestion.scenario);
}
function applyExample() {
  const example = examples.value[Number(selectedExample.value)];
  if (!example || selectedExample.value === '') return;
  applyingInputs = true;
  intentionalEntry.value = false;
  savedScenario.value = '';
  selectedSuggestion.value = '';
  exploratory.value = false;
  version.value = 0;
  mode.value = 'full';
  node.value = '';
  customerID.value = '';
  name.value = example.name;
  start.value = example.start_time.slice(0, 19);
  customer.value = JSON.stringify(example.customer, null, 2);
  entry.value = JSON.stringify(example.entry_event, null, 2);
  events.value = JSON.stringify(example.events || [], null, 2);
  changes.value = JSON.stringify(example.attribute_changes || [], null, 2);
  assertions.value = JSON.stringify(example.assertions, null, 2);
  faults.value = '[]';
  prior.value = 0;
  run.value = undefined;
  path.value = undefined;
  applyingInputs = false;
  defaultInputs = formSignature();
}
const journeys = ref<Journey[]>([]);
const journey = ref(props.journeyId || '');
const version = ref(0);
const name = ref('Customer journey scenario');
const start = ref(new Date().toISOString().slice(0, 19));
const mode = ref(props.nodeId ? 'single_node' : 'full');
const node = ref(props.nodeId || '');
watch(
  () => props.nodeId,
  (id, previous) => {
    if (id === previous) return;
    mode.value = id ? 'single_node' : 'full';
    node.value = id || '';
    run.value = undefined;
    path.value = undefined;
    selectedExample.value = '';
  },
);
const customerID = ref('');
const profile = ref('Custom Test Customer');
const customer = ref(
  JSON.stringify(
    {
      email: 'sandbox@example.invalid',
      phone: '+905551234567',
      push_token: 'sandbox-device-token',
      locale: 'en',
      timezone: 'Europe/Istanbul',
      attributes: { first_name: 'Ada', purchased: false, total_orders: 0, cart_amount: 2450 },
      consents: { email: true, sms: true, push: true },
    },
    null,
    2,
  ),
);
const entry = ref('{"event_id":"sandbox_entry","event_name":"","properties":{}}');
const events = ref('[]');
const changes = ref('[]');
const assertions = ref(
  '[{"type":"state","expected":"COMPLETED"},{"type":"no_duplicate_delivery","expected":true}]',
);
const faults = ref('[]');
const faultChannel = ref('email');
const prior = ref(0);
const scenarios = ref<{ id: string; name: string; journey_id: string; required: boolean }[]>([]);
const exploratory = ref(false);
const requiredScenario = ref(true);
async function changeRequirement() {
  await perform(async () => {
    await api(`/test-scenarios/${savedScenario.value}/requirement`, 'PATCH', {
      required: requiredScenario.value,
    });
    await refresh();
  });
}
const savedScenario = ref('');
watch(savedScenario, (id) => {
  requiredScenario.value = scenarios.value.find((s) => s.id === id)?.required ?? true;
});
watch(formSignature, () => {
  if (selectedSuggestion.value && !applyingInputs) reviewedSuggestion.value = false;
});
const currentDraft = ref(false);
const runs = ref<TestRunSummary[]>([]);
const run = ref<SandboxRun>();
const path = ref<RunState>();
const error = ref('');
const notice = ref('');
const busy = ref(false);
const state = computed(() => path.value || run.value?.state);
const versions = ref<{ version: number }[]>([]);
const outcome = ref('any');
const expectedMessages = ref('any');
const entryName = computed({
  get() {
    try {
      return JSON.parse(entry.value).event_name || '';
    } catch {
      return '';
    }
  },
  set(value: string) {
    try {
      entry.value = JSON.stringify({ ...JSON.parse(entry.value), event_name: value }, null, 2);
    } catch {
      error.value = 'Correct the advanced entry-event JSON first.';
    }
  },
});
const eventRows = computed(() => {
  try {
    return JSON.parse(events.value) as {
      after_seconds: number;
      event: { event_name: string; event_id?: string };
    }[];
  } catch {
    return [];
  }
});
function addEvent() {
  const rows = [
    ...eventRows.value,
    {
      after_seconds: 3600,
      event: { event_name: context.value?.eventNames[0] || '', event_id: crypto.randomUUID() },
    },
  ];
  events.value = JSON.stringify(rows, null, 2);
}
function changeEvent(index: number, key: string, value: string | number) {
  const rows = JSON.parse(events.value);
  if (key === 'hours') rows[index].after_seconds = Math.round(Number(value) * 3600);
  else rows[index].event.event_name = String(value);
  events.value = JSON.stringify(rows, null, 2);
}
function removeEvent(index: number) {
  events.value = JSON.stringify(
    eventRows.value.filter((_, i) => i !== index),
    null,
    2,
  );
}
watch(
  [outcome, expectedMessages],
  () => {
    if (applyingInputs) return;
    const checks: Record<string, unknown>[] = [{ type: 'no_duplicate_delivery', expected: true }];
    if (outcome.value !== 'any') checks.push({ type: 'state', expected: outcome.value });
    if (mode.value === 'full') checks.push({ type: 'enrollment_count', expected: 1 });
    if (outcome.value !== 'any' || expectedMessages.value !== 'any') exploratory.value = false;
    if (expectedMessages.value === 'none') checks.push({ type: 'no_message' });
    if (['email', 'both'].includes(expectedMessages.value))
      checks.push({ type: 'message_sent', channel: 'email' });
    if (['sms', 'both'].includes(expectedMessages.value))
      checks.push({ type: 'message_sent', channel: 'sms' });
    if (expectedMessages.value === 'push') checks.push({ type: 'message_sent', channel: 'push' });
    assertions.value = JSON.stringify(checks, null, 2);
  },
  { flush: 'sync' },
);
async function loadVersions() {
  if (!journey.value) {
    versions.value = [];
    return;
  }
  try {
    versions.value = await api('/journeys/' + journey.value + '/versions');
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
watch(journey, () => {
  version.value = 0;
  savedScenario.value = '';
  run.value = undefined;
  path.value = undefined;
  void loadVersions();
  void loadExamples();
});
watch(version, () => void loadExamples(false));
onActivated(() => {
  if (context.value) void loadExamples(false);
});
function preset() {
  const data = JSON.parse(customer.value);
  data.attributes.total_orders = profile.value === 'Returning Customer' ? 3 : 0;
  data.attributes.inactive = profile.value === 'Inactive Customer';
  if (profile.value === 'New Customer') data.attributes.first_name = 'New customer';
  customer.value = JSON.stringify(data, null, 2);
}
function input() {
  return {
    journey_id: journey.value || undefined,
    version: version.value,
    scenario_id: savedScenario.value || undefined,
    use_current_draft: currentDraft.value,
    exploratory: exploratory.value,
    required: requiredScenario.value,
    ...(savedScenario.value
      ? {}
      : {
          scenario: {
            name: name.value,
            mode: mode.value,
            node_id: node.value || undefined,
            start_time: new Date(start.value + 'Z').toISOString(),
            customer_id: customerID.value || undefined,
            customer: JSON.parse(customer.value),
            entry_event: JSON.parse(entry.value),
            events: JSON.parse(events.value),
            attribute_changes: JSON.parse(changes.value),
            assertions: JSON.parse(assertions.value),
            provider_faults: JSON.parse(faults.value),
            fault_channel: faultChannel.value,
            prior_messages: prior.value,
          },
        }),
  };
}
async function refresh() {
  [scenarios.value, runs.value] = await Promise.all([
    api<typeof scenarios.value>('/test-scenarios'),
    api<typeof runs.value>('/test-runs' + (journey.value ? '?journey_id=' + journey.value : '')),
  ]);
}
async function perform(action: () => Promise<void>) {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await action();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
async function prepare() {
  if (props.beforeRun) await props.beforeRun();
  if (savedScenario.value) return;
  await loadExamples(false);
  if (sourceFailed.value || !context.value?.entryName)
    throw new Error('Load a workflow with an Event Trigger before testing.');
  if (mismatchedEntry.value && !intentionalEntry.value)
    throw new Error(
      `This journey starts with ${context.value.entryName}. Update the entry event or mark this as an intentional negative-entry test.`,
    );
}
async function create() {
  await perform(async () => {
    await prepare();
    run.value = await api<SandboxRun>('/test-runs', 'POST', input());
    path.value = undefined;
    await refresh();
  });
}
async function saveScenario() {
  await perform(async () => {
    if (selectedSuggestion.value && !reviewedSuggestion.value)
      throw new Error(
        'Review the suggested inputs and expected checks before saving a required scenario.',
      );
    await prepare();
    await api('/test-scenarios', 'POST', input());
    notice.value = 'Scenario saved with workflow and customer snapshots';
    await refresh();
  });
}
async function preview() {
  await perform(async () => {
    await prepare();
    path.value = await api<RunState>('/preview/journey', 'POST', input());
  });
}
async function advance(operation: string, seconds = 0) {
  await perform(async () => {
    if (!run.value) return;
    path.value = undefined;
    run.value = await api<SandboxRun>(`/test-runs/${run.value.id}/advance`, 'POST', {
      operation,
      seconds,
      revision: run.value.revision,
    });
    await refresh();
  });
}
async function replay() {
  await perform(async () => {
    if (!run.value) return;
    run.value = await api<SandboxRun>(`/test-runs/${run.value.id}/replay`, 'POST');
    path.value = undefined;
    await refresh();
  });
}
async function open(id: string) {
  await perform(async () => {
    run.value = await api<SandboxRun>('/test-runs/' + id);
    path.value = undefined;
  });
}
onMounted(
  () =>
    void perform(async () => {
      if (!props.journeyId) journeys.value = await api<Journey[]>('/journeys?limit=200');
      await refresh();
      await loadVersions();
      await loadExamples();
    }),
);
</script>
<template>
  <section class="test-lab">
    <div class="page-heading">
      <div>
        <h1>Test Lab</h1>
        <p class="page-description">
          Try a customer scenario and fast-forward time before going live.
        </p>
      </div>
      <span class="sandbox-badge"><AppIcon name="flask" :size="15" />Isolated sandbox</span>
    </div>
    <p v-if="error" class="inline-alert error-alert" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-message" role="status">{{ notice }}</p>
    <div class="test-lab-layout">
      <aside class="surface scenario-panel">
        <div class="surface-heading">
          <h2>Scenario setup</h2>
          <AppIcon name="settings" :size="16" />
        </div>
        <fieldset class="scenario-form" :disabled="sourceLoading">
          <p v-if="sourceFailed" class="error-message">
            Journey inputs could not be loaded. Your values are preserved.
            <button type="button" @click="loadExamples(false)">Reload journey inputs</button>
          </p>
          <SkeletonBlock v-if="sourceLoading" label="Loading journey inputs…" variant="text" />
          <section
            v-if="context && !savedScenario"
            class="journey-test-context"
            aria-label="Journey test context"
          >
            <strong>Starts with {{ context.entryName }}</strong>
            <p>Test-only values prepared from this workflow. Review them before running.</p>
            <p v-if="sourceChanged">Workflow changed. Your inputs have been kept.</p>
            <button type="button" class="text-button" @click="applyDefaults">
              Use journey defaults
            </button>
            <ul v-if="context.notes.length">
              <li v-for="note in context.notes" :key="note">{{ note }}</li>
            </ul>
          </section>
          <section v-if="context && !savedScenario && !examples.length" class="suggested-checks">
            <label
              >Suggested check<AppSelect
                v-model="selectedSuggestion"
                aria-label="Suggested check"
                :options="[
                  { value: '', label: 'Custom inputs' },
                  ...context.suggestions.map((suggestion, index) => ({
                    value: String(index),
                    label: suggestion.label,
                  })),
                ]"
                @change="applySuggestion" /></label
            ><template v-if="selectedSuggestion !== ''"
              ><p>{{ context.suggestions[Number(selectedSuggestion)]?.note }}</p>
              <label class="checkbox-label"
                ><input
                  v-model="reviewedSuggestion"
                  type="checkbox"
                  @change="reviewedSuggestion && (exploratory = false)"
                />I reviewed the sample inputs and expected checks</label
              ></template
            >
          </section>
          <div v-if="examples.length" class="example-scenarios">
            <label
              >Recovery scenario<AppSelect
                v-model="selectedExample"
                aria-label="Recovery scenario"
                :disabled="busy || readonly"
                :options="[
                  { value: '', label: 'Custom inputs' },
                  ...examples.map((example, index) => ({
                    value: String(index),
                    label: example.name,
                  })),
                ]"
                @change="applyExample"
            /></label>
            <p>Synthetic Alex · €89.90 cart. Presets fill editable sandbox inputs.</p>
            <details>
              <summary>Journey rules &amp; assumptions</summary>
              <ul>
                <li v-for="note in exampleNotes" :key="note">{{ note }}</li>
              </ul>
            </details>
          </div>
          <label v-if="!journeyId"
            >Journey<AppSelect
              v-model="journey"
              :options="[
                { value: '', label: 'Select a journey' },
                ...journeys.map((j) => ({ value: j.id, label: j.name })),
              ]"
              @change="refresh" /></label
          ><label
            >Saved scenario<AppSelect
              v-model="savedScenario"
              :options="[
                { value: '', label: 'Create a new scenario' },
                ...scenarios
                  .filter((s) => !journey || s.journey_id === journey)
                  .map((s) => ({ value: s.id, label: s.name })),
              ]" /></label
          ><label
            >Workflow version<AppSelect
              v-model="version"
              :options="[
                { value: 0, label: 'Current draft' },
                ...versions.map((v) => ({ value: v.version, label: 'Published v' + v.version })),
              ]" /></label
          ><label v-if="savedScenario" class="checkbox-label"
            ><input v-model="currentDraft" type="checkbox" />Use current draft for regression</label
          >
          <label class="checkbox-label"
            ><input v-model="exploratory" :disabled="busy || readonly" type="checkbox" />Exploratory
            run (does not qualify for publishing)</label
          >
          <label class="checkbox-label"
            ><input
              v-model="requiredScenario"
              :disabled="busy || readonly"
              type="checkbox"
              @change="savedScenario && changeRequirement()"
            />Saved scenario required for publishing</label
          >
          <div v-if="savedScenario" class="scenario-snapshot">
            <AppIcon name="history" :size="17" />
            <p>Uses the saved workflow and customer snapshot. Replay keeps the original inputs.</p>
          </div>
          <template v-else
            ><label>Scenario name<input v-model="name" maxlength="160" /></label>
            <div class="form-section-title">Test customer</div>
            <label
              >Profile<AppSelect
                v-model="profile"
                :options="[
                  {
                    value: 'Custom Test Customer',
                    label: 'Custom',
                    description: 'Edit every field yourself',
                  },
                  {
                    value: 'Existing Customer',
                    label: 'Existing customer',
                    description: 'Copy a profile from this workspace',
                  },
                  {
                    value: 'New Customer',
                    label: 'New customer',
                    description: 'Signed up today, no orders',
                  },
                  {
                    value: 'Returning Customer',
                    label: 'Returning customer',
                    description: 'Has ordered before',
                  },
                  {
                    value: 'Inactive Customer',
                    label: 'Inactive customer',
                    description: 'No activity for months',
                  },
                ]"
                @change="preset" /></label
            ><label v-if="profile === 'Existing Customer'"
              >Source customer ID<input v-model="customerID" placeholder="Customer ID" /></label
            ><CustomerProfileFields v-model="customer" />
            <ScenarioFields v-model="customer" :fields="customerFields" root="customer" />
            <div class="form-section-title">Entry &amp; events</div>
            <label
              >Entry event name<input
                v-model="entryName"
                placeholder="Event that starts this journey"
            /></label>
            <p v-if="mismatchedEntry" class="warning-message">
              This journey expects {{ context?.entryName }}. This event will not enter the journey.
            </p>
            <label v-if="mismatchedEntry" class="checkbox-label"
              ><input v-model="intentionalEntry" type="checkbox" />Intentional non-matching entry
              test</label
            >
            <ScenarioFields v-model="entry" :fields="eventFields" root="event" />
            <div v-for="(e, i) in eventRows" :key="i" class="scenario-event">
              <label
                >Event name<input
                  :value="e.event.event_name"
                  @input="changeEvent(i, 'name', ($event.target as HTMLInputElement).value)"
              /></label>
              <div>
                <label
                  >After (hours)<input
                    :value="e.after_seconds / 3600"
                    type="number"
                    min="0"
                    step="0.25"
                    @input="
                      changeEvent(i, 'hours', Number(($event.target as HTMLInputElement).value))
                    " /></label
                ><button class="icon-button" aria-label="Remove event" @click="removeEvent(i)">
                  <AppIcon name="close" :size="15" />
                </button>
              </div>
            </div>
            <button class="add-event-button" @click="addEvent">
              <AppIcon name="plus" :size="15" />Add event
            </button>
            <div class="form-section-title">Expected results</div>
            <p v-if="selectedExample !== ''" class="card-footnote">
              This preset includes its own message, conversion and re-entry assertions. Review or
              edit them under Advanced scenario options.
            </p>
            <template v-else>
              <label
                >Expected outcome<AppSelect
                  v-model="outcome"
                  :options="[
                    {
                      value: 'any',
                      label: 'Not checked',
                      description: 'Pick the outcome your brief expects',
                    },
                    { value: 'COMPLETED', label: 'Completes', tone: 'blue' },
                    { value: 'EXITED', label: 'Exits early', tone: 'purple' },
                    { value: 'FAILED', label: 'Fails', tone: 'red' },
                    { value: 'WAITING', label: 'Still waiting', tone: 'amber' },
                  ]" /></label
              ><label
                >Expected messages<AppSelect
                  v-model="expectedMessages"
                  :options="[
                    {
                      value: 'any',
                      label: 'Default checks',
                      description: 'Whatever the journey sends',
                    },
                    { value: 'both', label: 'Email and SMS' },
                    { value: 'email', label: 'Email only', icon: 'email' },
                    { value: 'sms', label: 'SMS only', icon: 'message' },
                    { value: 'push', label: 'Push sent', icon: 'push' },
                    { value: 'none', label: 'Nothing sent' },
                  ]"
              /></label>
            </template>
            <details class="advanced-options">
              <summary>Advanced scenario options <AppIcon name="down" :size="13" /></summary>
              <div class="editor-form">
                <label
                  >Start time (UTC)<input v-model="start" type="datetime-local" step="1" /></label
                ><SegmentedControl
                  v-model="mode"
                  label="Test mode"
                  :options="[
                    { value: 'full', label: 'Whole journey' },
                    { value: 'single_node', label: 'One step' },
                  ]"
                /><label v-if="mode === 'single_node'">Node ID<input v-model="node" /></label
                ><label
                  >Prior sandbox messages<input
                    v-model.number="prior"
                    type="number"
                    min="0"
                    max="1000" /></label
                ><SegmentedControl
                  v-model="faultChannel"
                  label="Fault channel"
                  :options="[
                    { value: 'email', label: 'Email', icon: 'email' },
                    { value: 'sms', label: 'SMS', icon: 'message' },
                    { value: 'push', label: 'Push', icon: 'push' },
                  ]"
                /><label
                  >Provider faults<AppSelect
                    v-model="faults"
                    :options="[
                      { value: '[]', label: 'Accepted', tone: 'green' },
                      {
                        value: '[429,500,200]',
                        label: 'Retried, then accepted',
                        description: '429, then 500, then accepted',
                        tone: 'amber',
                      },
                      {
                        value: '[408]',
                        label: 'Timed out',
                        description: 'The provider may or may not have sent it',
                        tone: 'amber',
                      },
                      {
                        value: '[400]',
                        label: 'Rejected',
                        description: 'Not retried',
                        tone: 'red',
                      },
                    ]" /></label
                ><label
                  >Customer overrides (JSON)<textarea
                    v-model="customer"
                    rows="8"
                    spellcheck="false"
                  /></label
                ><label
                  >Entry event (JSON)<textarea v-model="entry" rows="5" spellcheck="false" /></label
                ><label
                  >Event sequence (JSON; after_seconds, event)<textarea
                    v-model="events"
                    rows="5"
                    spellcheck="false"
                  /></label
                ><label
                  >Attribute changes (JSON; after_seconds, attributes)<textarea
                    v-model="changes"
                    rows="5"
                    spellcheck="false"
                  /></label
                ><label
                  >Assertions (JSON)<textarea v-model="assertions" rows="5" spellcheck="false" />
                </label>
              </div></details
          ></template>
          <div class="scenario-actions">
            <button
              class="primary"
              :disabled="
                busy ||
                readonly ||
                (sourceFailed && !savedScenario) ||
                (!journey && !savedScenario) ||
                (mismatchedEntry && !intentionalEntry)
              "
              @click="create"
            >
              <AppIcon name="play" :size="15" />Create test run</button
            ><button :disabled="busy || readonly" @click="saveScenario">
              <AppIcon name="file" :size="15" />Save scenario</button
            ><button class="text-button" :disabled="busy" @click="preview">
              Preview journey path <AppIcon name="arrow" :size="14" />
            </button>
          </div>
          <p v-if="createBlocker" class="action-hint" role="status">{{ createBlocker }}</p>
        </fieldset>
      </aside>
      <div class="test-results-column">
        <VirtualClock
          v-if="run"
          :state="run.state"
          :disabled="busy || readonly"
          @advance="advance"
          @replay="replay"
        />
        <SandboxResult :state="state" :preview="!!path" />
        <section
          v-if="run && state?.messages.length"
          class="surface run-inbox"
          aria-label="Test run inbox"
        >
          <TestInbox
            :key="run.id"
            embedded
            :test-run-id="run.id"
            :run-revision="run.revision"
            :readonly="readonly"
          />
        </section>
        <RecentTestRuns :runs="runs" @open="open" />
      </div>
    </div>
  </section>
</template>
