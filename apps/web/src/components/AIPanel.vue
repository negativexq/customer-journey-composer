<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import { errorMessage } from '../lib/format';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { api, generationStream, type GenerationProgress } from '../lib/api';
import GenerationStatus from './GenerationStatus.vue';
import DataMappingReview from './DataMappingReview.vue';
import JourneyDiagram from './JourneyDiagram.vue';
import BehaviorChecksEditor from './BehaviorChecksEditor.vue';
import BehaviorChecksSummary from './BehaviorChecksSummary.vue';
import { behaviorChecksReady } from '../lib/behavior-checks';
import { layoutGeneratedWorkflow } from '../lib/generated-layout';
import type { Workflow, BehaviorChecks } from '../lib/workflow';
const props = defineProps<{
  workflow: Workflow;
  name: string;
  description: string;
  readonly?: boolean;
}>();
type Diff = {
  behavior_checks_changed?: boolean;
  metadata_changed: boolean;
  added_nodes: string[];
  removed_nodes: string[];
  updated_nodes: string[];
  changed_conditions: string[];
  changed_messages: string[];
  edges_changed: boolean;
  exit_changed: boolean;
  reentry_changed: boolean;
};
type Proposal = {
  generation?: {
    attempts: { model?: string; requested_model: string; duration_ms: number; usage: unknown }[];
    usage: { input_tokens: number; output_tokens: number; reasoning_tokens: number | null } | null;
  };
  schema_version: string;
  name: string;
  description: string;
  workflow: Workflow;
  explanation: string;
  warnings: string[];
  not_done?: string[];
  diff: Diff;
  action: string;
};
const emit = defineEmits<{
  apply: [Proposal];
  progress: [{ progress: GenerationProgress; elapsed: number } | undefined];
}>();
const status = ref<{
  enabled: boolean;
  default_provider: string;
  providers: Record<string, { configured: boolean; model: string }>;
}>();
const behaviorChecks = ref<BehaviorChecks | undefined>(
  props.workflow.behavior_checks
    ? JSON.parse(JSON.stringify(props.workflow.behavior_checks))
    : undefined,
);
const acceptanceContract = ref(props.workflow.acceptance_contract || '');
const provider = ref('claude');
const action = ref(props.workflow.nodes.length ? 'modify' : 'generate');
const prompt = ref('');
const revising = ref(false);
const revisionBase = ref<Proposal>();
let revisionBaseline = '';
let previousPrompt = '';
function requestChanges() {
  if (!proposal.value || stale.value) return;
  revisionBase.value = proposal.value;
  revisionBaseline = baseline;
  previousPrompt = prompt.value;
  prompt.value = '';
  revising.value = true;
}
function cancelChanges() {
  revising.value = false;
  if (revisionBase.value) proposal.value = revisionBase.value;
  revisionBase.value = undefined;
  prompt.value = previousPrompt;
}
const nodeID = ref('');
const busy = ref(false);
const error = ref('');
const progress = ref<GenerationProgress>({ stage: 'sending' });
const elapsed = ref(0);
let timer: ReturnType<typeof setInterval> | undefined;
function broadcast() {
  emit('progress', busy.value ? { progress: progress.value, elapsed: elapsed.value } : undefined);
}
function cancel() {
  controller?.abort();
}
defineExpose({ cancel });
function leaving(event: BeforeUnloadEvent) {
  if (busy.value) event.preventDefault();
}
const notice = ref('');
const loadingBrief = ref(false);
async function useRecoveryBrief() {
  loadingBrief.value = true;
  try {
    const sample = await api<{ brief: string }>('/templates/abandoned-cart-eur');
    prompt.value = sample.brief;
    action.value = 'generate';
    acceptanceContract.value = 'cart-recovery-eur.v1';
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    loadingBrief.value = false;
  }
}
const proposal = ref<Proposal>();
const acknowledged = ref(false);
const dataReady = ref(false);
const confirmedReferences = ref<string[]>([]);
const inspect = ref(false);
let baseline = '';
let controller: AbortController | undefined;
const snapshot = () =>
  JSON.stringify([props.name, props.description, props.workflow, behaviorChecks.value]);
const stale = computed(() => !!proposal.value && baseline !== snapshot());
const configured = computed(
  () => !!status.value?.enabled && !!status.value.providers[provider.value]?.configured,
);
const needsNode = computed(() =>
  ['explain_node', 'improve_message', 'generate_message'].includes(action.value),
);
// Message actions work on an email or SMS step only; explaining works on any.
const stepOptions = computed(() =>
  props.workflow.nodes
    .filter(
      (node) => action.value === 'explain_node' || node.type === 'email' || node.type === 'sms',
    )
    .map((node) => ({ value: node.id, label: node.label })),
);
watch(stepOptions, (options) => {
  if (nodeID.value && !options.some((option) => option.value === nodeID.value)) nodeID.value = '';
});
const changed = computed(
  () => proposal.value && action.value !== 'explain_journey' && action.value !== 'explain_node',
);
function label(id: string) {
  return (
    proposal.value?.workflow.nodes.find((n) => n.id === id)?.label ||
    props.workflow.nodes.find((n) => n.id === id)?.label ||
    id
  );
}
async function compose() {
  if (busy.value || props.readonly || !configured.value || (revising.value && !prompt.value.trim()))
    return;
  if (
    !['explain_journey', 'explain_node', 'improve_message', 'generate_message'].includes(
      action.value,
    ) &&
    !acceptanceContract.value &&
    !behaviorChecksReady(behaviorChecks.value)
  ) {
    error.value = 'Review the independent behavior checks before generating.';
    return;
  }
  busy.value = true;
  progress.value = { stage: 'sending' };
  elapsed.value = 0;
  const started = Date.now();
  broadcast();
  timer = setInterval(() => {
    elapsed.value = Math.floor((Date.now() - started) / 1000);
    broadcast();
  }, 1000);
  error.value = '';
  notice.value = '';
  proposal.value = undefined;
  acknowledged.value = false;
  dataReady.value = false;
  confirmedReferences.value = [];
  baseline = revising.value ? revisionBaseline : snapshot();
  controller?.abort();
  const request = new AbortController();
  controller = request;
  try {
    const input = {
      behavior_checks: acceptanceContract.value ? undefined : behaviorChecks.value,
      provider: provider.value,
      acceptance_contract: acceptanceContract.value || undefined,
      action: revising.value ? 'modify' : action.value,
      prompt: prompt.value.trim() || action.value.replaceAll('_', ' '),
      name: props.name,
      description: props.description,
      workflow: revisionBase.value?.workflow || props.workflow,
      node_id: nodeID.value || undefined,
    };
    const path = ['improve_message', 'generate_message'].includes(action.value)
      ? '/ai/messages/generate'
      : !revising.value && action.value === 'generate'
        ? '/ai/journeys/generate'
        : '/ai/journeys/modify';
    const result = await generationStream<Proposal>(
      path,
      input,
      (event) => {
        progress.value = event;
        broadcast();
      },
      request.signal,
    );
    if (!request.signal.aborted) {
      if (
        !['explain_journey', 'explain_node', 'improve_message', 'generate_message'].includes(
          action.value,
        )
      )
        result.workflow = layoutGeneratedWorkflow(
          result.workflow,
          action.value === 'generate' ? undefined : props.workflow,
        );
      proposal.value = result;
      revising.value = false;
      revisionBase.value = undefined;
    }
  } catch (cause) {
    error.value = request.signal.aborted
      ? 'Cancellation requested. Your draft is unchanged; the provider may already have processed the request.'
      : errorMessage(cause);
  } finally {
    busy.value = false;
    clearInterval(timer);
    broadcast();
  }
}
// Warnings and what the request asked but the draft left out both need a look.
const needsAcknowledgement = computed(
  () =>
    !!proposal.value && proposal.value.warnings.length + (proposal.value.not_done?.length ?? 0) > 0,
);
async function apply() {
  if (
    props.readonly ||
    busy.value ||
    revising.value ||
    !proposal.value ||
    stale.value ||
    !dataReady.value ||
    (needsAcknowledgement.value && !acknowledged.value)
  )
    return;
  busy.value = true;
  error.value = '';
  try {
    const candidate: Proposal = JSON.parse(JSON.stringify(proposal.value));
    if (!['improve_message', 'generate_message'].includes(action.value)) {
      if (!acceptanceContract.value) candidate.workflow.behavior_checks = behaviorChecks.value;
      const result = await api<{ valid: boolean; issues: { message: string }[] }>(
        '/workflows/validate',
        'POST',
        candidate.workflow,
      );
      if (!result.valid) {
        error.value = result.issues.map((issue) => issue.message).join(' · ');
        return;
      }
      if (stale.value) {
        error.value = 'The draft or expectations changed. Review a fresh proposal.';
        return;
      }
    }
    emit('apply', candidate);
    proposal.value = undefined;
    notice.value = 'Proposal applied to the draft. Publish remains a separate action.';
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
function discard() {
  revising.value = false;
  revisionBase.value = undefined;
  proposal.value = undefined;
  notice.value = 'Proposal discarded. Your draft is unchanged.';
}
onMounted(async () => {
  window.addEventListener('beforeunload', leaving);
  try {
    status.value = await api('/settings/ai');
    if (status.value?.default_provider === 'openai') provider.value = 'openai';
  } catch (cause) {
    error.value = errorMessage(cause);
  }
});
onBeforeUnmount(() => {
  controller?.abort();
  clearInterval(timer);
  window.removeEventListener('beforeunload', leaving);
  emit('progress', undefined);
});
</script>
<template>
  <aside class="ai-panel panel" aria-label="AI journey composer">
    <div class="card-top">
      <h2><span aria-hidden="true">✧</span> AI composer</h2>
      <span class="pill">DRAFT ONLY</span>
    </div>
    <p>Describe the experience. Review each change before applying it.</p>
    <div class="ai-starter">
      <span>START WITH A BRIEF</span
      ><button :disabled="busy || readonly || loadingBrief" @click="useRecoveryBrief">
        Recover a €50+ abandoned cart <span aria-hidden="true">↗</span>
      </button>
      <p>1h email · 24h SMS window · 48h follow-up</p>
    </div>
    <p v-if="status && !configured" class="warning-message">
      Live generation is disabled or the selected provider needs a server-side API key and model.
      Manual editing and tests remain available.
    </p>
    <details open class="composer-options">
      <summary>AI options</summary>
      <SegmentedControl
        v-model="provider"
        label="Provider"
        :disabled="busy"
        :options="[
          { value: 'claude', label: 'Claude' },
          { value: 'openai', label: 'OpenAI' },
        ]"
      />

      <label
        >Behavior contract<AppSelect
          v-model="acceptanceContract"
          :disabled="busy || readonly || !!workflow.acceptance_contract"
          :options="[
            {
              value: '',
              label: 'General workflow',
              description: 'You write and review the acceptance tests',
            },
            {
              value: 'cart-recovery-eur.v1',
              label: 'EUR cart recovery',
              description: 'Checked against the validated recovery behavior',
            },
          ]"
      /></label>
      <label v-if="!revising"
        >Action<AppSelect
          v-model="action"
          :disabled="busy"
          :options="[
            { value: 'generate', label: 'Draft a new journey', icon: 'sparkles', group: 'Journey' },
            { value: 'modify', label: 'Change this journey', icon: 'workflow', group: 'Journey' },
            { value: 'add_step', label: 'Add a step', icon: 'plus', group: 'Journey' },
            {
              value: 'fix_validation',
              label: 'Fix validation errors',
              icon: 'shield',
              group: 'Journey',
            },
            {
              value: 'generate_message',
              label: 'Write a message',
              icon: 'email',
              group: 'Messages',
            },
            {
              value: 'improve_message',
              label: 'Improve a message',
              icon: 'message',
              group: 'Messages',
            },
            {
              value: 'explain_journey',
              label: 'Explain the journey',
              icon: 'file',
              group: 'Explain',
            },
            { value: 'explain_node', label: 'Explain a step', icon: 'file', group: 'Explain' },
          ]" /></label
      ><label v-if="needsNode"
        >Step<AppSelect v-model="nodeID" placeholder="Choose a step" :options="stepOptions"
      /></label>
    </details>
    <BehaviorChecksEditor
      v-if="
        !acceptanceContract &&
        !['explain_journey', 'explain_node', 'improve_message', 'generate_message'].includes(action)
      "
      v-model="behaviorChecks"
      :disabled="busy || readonly"
    />
    <label
      >{{ revising ? 'What should change?' : 'What should happen?'
      }}<textarea
        v-model="prompt"
        rows="6"
        maxlength="4000"
        placeholder="When a cart is updated, wait 2 hours. Email if there is no purchase. Exit immediately on purchase."
        :disabled="busy || readonly"
      /></label
    ><small
      >Your prompt, draft and reviewed expected results are sent. Synthetic example values and live
      customer records are excluded.</small
    ><button
      class="primary"
      :disabled="busy || readonly || !configured || (needsNode && !nodeID)"
      @click="compose"
    >
      {{ busy ? 'Composing proposal…' : revising ? 'Update proposal' : 'Compose proposal' }}
    </button>
    <button v-if="revising && !busy" type="button" @click="cancelChanges">Cancel changes</button>
    <GenerationStatus v-if="busy" :progress="progress" :elapsed="elapsed" @cancel="cancel" />
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <section v-if="proposal" class="ai-proposal">
      <h3>Review proposal</h3>
      <p>Proposed plan · not applied or published</p>
      <BehaviorChecksSummary :checks="proposal.workflow.behavior_checks" />
      <JourneyDiagram :workflow="proposal.workflow" />
      <p v-if="proposal.generation" class="help-text">
        Provider attempts: {{ proposal.generation.attempts.length }}.
        <template v-if="proposal.generation.usage"
          >Reported tokens: {{ proposal.generation.usage.input_tokens }} input /
          {{ proposal.generation.usage.output_tokens }} output. Reasoning:
          {{ proposal.generation.usage.reasoning_tokens ?? 'not reported' }}.</template
        >
        <template v-else>Complete token usage was not reported; totals are unknown.</template>
      </p>
      <p>{{ proposal.explanation }}</p>
      <dl class="proposal-intent">
        <div>
          <dt>Entry event</dt>
          <dd>
            {{
              proposal.workflow.nodes.find((n) => n.type === 'event_trigger')?.config.event_name ||
              'Not specified'
            }}
          </dd>
        </div>
        <div>
          <dt>Steps</dt>
          <dd>{{ proposal.workflow.nodes.length }} proposed</dd>
        </div>
        <div>
          <dt>Exit on</dt>
          <dd>{{ proposal.workflow.global_exit?.event_name || 'Configured exit steps' }}</dd>
        </div>
        <div>
          <dt>Re-entry</dt>
          <dd>
            {{
              proposal.workflow.reentry?.mode === 'cooldown'
                ? 'After ' + (proposal.workflow.reentry.cooldown_seconds || 0) / 86400 + ' days'
                : proposal.workflow.reentry?.mode?.replaceAll('_', ' ') || 'After completion'
            }}
          </dd>
        </div>
      </dl>
      <template v-if="changed"
        ><h4>{{ proposal.name }}</h4>
        <p>{{ proposal.description }}</p>
        <div class="diff-summary">
          <p v-if="proposal.diff.metadata_changed">Journey metadata changed</p>
          <p>Added nodes: {{ proposal.diff.added_nodes.map(label).join(', ') || 'None' }}</p>
          <p>Removed nodes: {{ proposal.diff.removed_nodes.map(label).join(', ') || 'None' }}</p>
          <p>Updated nodes: {{ proposal.diff.updated_nodes.map(label).join(', ') || 'None' }}</p>
          <p>
            Changed conditions:
            {{ proposal.diff.changed_conditions.map(label).join(', ') || 'None' }}
          </p>
          <p>
            Changed messages: {{ proposal.diff.changed_messages.map(label).join(', ') || 'None' }}
          </p>
          <p v-if="proposal.diff.behavior_checks_changed">Independent behavior checks changed</p>
          <p v-if="proposal.diff.edges_changed">Connections changed</p>
          <p v-if="proposal.diff.exit_changed">Global exit changed</p>
          <p v-if="proposal.diff.reentry_changed">Re-entry policy changed</p>
        </div>
        <button @click="inspect = !inspect">
          {{ inspect ? 'Hide' : 'Inspect' }} proposed workflow
        </button>
        <pre v-if="inspect">{{ JSON.stringify(proposal.workflow, null, 2) }}</pre>
        <p v-if="stale" class="error-message">
          Your draft changed after this request. Discard and compose again to avoid overwriting
          newer edits.
        </p></template
      >
      <DataMappingReview
        v-if="changed"
        :workflow="proposal.workflow"
        :confirmed="confirmedReferences"
        :readonly="readonly"
        @ready="dataReady = $event"
        @resolved="
          (workflow, confirmed) => {
            proposal!.workflow = workflow;
            confirmedReferences = confirmed;
            acknowledged = false;
          }
        "
      />
      <section v-if="proposal.not_done?.length" class="not-done" aria-labelledby="ai-not-done">
        <h3 id="ai-not-done">In the request, not in the draft</h3>
        <ul>
          <li v-for="note in proposal.not_done" :key="note">{{ note }}</li>
        </ul>
      </section>
      <ul v-if="proposal.warnings.length">
        <li v-for="warning in proposal.warnings" :key="warning" class="warning-message">
          {{ warning }}
        </li>
      </ul>
      <label v-if="changed && needsAcknowledgement" class="acknowledge"
        ><input v-model="acknowledged" type="checkbox" /> I reviewed the warnings and
        assumptions</label
      >
      <div class="toolbar">
        <button
          v-if="changed"
          class="primary"
          :disabled="
            readonly || revising || stale || !dataReady || (needsAcknowledgement && !acknowledged)
          "
          @click="apply"
        >
          Apply proposal</button
        ><button v-if="changed" type="button" :disabled="readonly || stale" @click="requestChanges">
          Request changes</button
        ><button :disabled="readonly" @click="discard">Discard proposal</button>
      </div>
    </section>
  </aside>
</template>
