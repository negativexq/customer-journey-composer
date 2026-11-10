<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import type { Workflow } from '../lib/workflow';
const props = defineProps<{
  workflow: Workflow;
  name: string;
  description: string;
  readonly?: boolean;
}>();
type Diff = {
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
  schema_version: string;
  name: string;
  description: string;
  workflow: Workflow;
  explanation: string;
  warnings: string[];
  diff: Diff;
  action: string;
};
const emit = defineEmits<{ apply: [Proposal] }>();
const status = ref<{
  enabled: boolean;
  default_provider: string;
  providers: Record<string, { configured: boolean; model: string }>;
}>();
const provider = ref('claude');
const action = ref(props.workflow.nodes.length ? 'modify' : 'generate');
const prompt = ref('');
const nodeID = ref('');
const busy = ref(false);
const error = ref('');
const notice = ref('');
const proposal = ref<Proposal>();
const acknowledged = ref(false);
const inspect = ref(false);
let baseline = '';
let controller: AbortController | undefined;
const snapshot = () => JSON.stringify([props.name, props.description, props.workflow]);
const stale = computed(() => !!proposal.value && baseline !== snapshot());
const configured = computed(
  () => !!status.value?.enabled && !!status.value.providers[provider.value]?.configured,
);
const needsNode = computed(() =>
  ['explain_node', 'improve_message', 'generate_message'].includes(action.value),
);
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
  if (props.readonly || !configured.value) return;
  busy.value = true;
  error.value = '';
  notice.value = '';
  proposal.value = undefined;
  acknowledged.value = false;
  baseline = snapshot();
  controller?.abort();
  const request = new AbortController();
  controller = request;
  try {
    const input = {
      provider: provider.value,
      action: action.value,
      prompt: prompt.value.trim() || action.value.replaceAll('_', ' '),
      name: props.name,
      description: props.description,
      workflow: props.workflow,
      node_id: nodeID.value || undefined,
    };
    const path = ['improve_message', 'generate_message'].includes(action.value)
      ? '/ai/messages/generate'
      : action.value === 'generate'
        ? '/ai/journeys/generate'
        : '/ai/journeys/modify';
    const result = await api<Proposal>(path, 'POST', input, {}, 90000, request.signal);
    if (!request.signal.aborted) proposal.value = result;
  } catch (cause) {
    error.value = request.signal.aborted ? 'Request cancelled' : String(cause);
  } finally {
    busy.value = false;
  }
}
function apply() {
  if (!proposal.value || stale.value || (proposal.value.warnings.length && !acknowledged.value))
    return;
  emit('apply', JSON.parse(JSON.stringify(proposal.value)));
  proposal.value = undefined;
  notice.value = 'Proposal applied to the draft. Publish remains a separate action.';
}
function discard() {
  proposal.value = undefined;
  notice.value = 'Proposal discarded. Your draft is unchanged.';
}
onMounted(async () => {
  try {
    status.value = await api('/settings/ai');
    if (status.value?.default_provider === 'openai') provider.value = 'openai';
  } catch (cause) {
    error.value = String(cause);
  }
});
onBeforeUnmount(() => controller?.abort());
</script>
<template>
  <aside class="ai-panel panel" aria-label="AI journey composer">
    <div class="card-top">
      <h2><span aria-hidden="true">✧</span> AI composer</h2>
      <span class="pill">DRAFT ONLY</span>
    </div>
    <p>Describe the experience. Review each change before applying it.</p>
    <label
      >Provider<select v-model="provider" :disabled="busy">
        <option value="claude">Claude</option>
        <option value="openai">OpenAI</option>
      </select></label
    >
    <p v-if="status && !configured" class="warning-message">
      Live generation is disabled or the selected provider needs a server-side API key and model.
      Manual editing and tests remain available.
    </p>
    <label
      >Action<select v-model="action" :disabled="busy">
        <option value="generate">Generate Journey</option>
        <option value="modify">Modify Journey</option>
        <option value="add_step">Add Step</option>
        <option value="explain_journey">Explain Journey</option>
        <option value="explain_node">Explain Node</option>
        <option value="improve_message">Improve Message</option>
        <option value="generate_message">Generate Message</option>
        <option value="fix_validation">Fix Validation Errors</option>
      </select></label
    ><label v-if="needsNode"
      >Selected node<select v-model="nodeID">
        <option value="">Choose a node</option>
        <option v-for="node in workflow.nodes" :key="node.id" :value="node.id">
          {{ node.label }}
        </option>
      </select></label
    ><label
      >What should happen?<textarea
        v-model="prompt"
        rows="6"
        maxlength="4000"
        placeholder="When a cart is updated, wait 2 hours. Email if there is no purchase. Exit immediately on purchase."
        :disabled="busy || readonly"
      /></label
    ><small
      >Only your prompt and draft are sent. Live customer profiles and event payloads are never
      attached.</small
    ><button
      class="primary"
      :disabled="busy || readonly || !configured || (needsNode && !nodeID)"
      @click="compose"
    >
      {{ busy ? 'Composing proposal…' : 'Compose proposal' }}</button
    ><button v-if="busy" @click="controller?.abort()">Cancel request</button>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <section v-if="proposal" class="ai-proposal">
      <h3>Review proposal</h3>
      <p>{{ proposal.explanation }}</p>
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
      <ul v-if="proposal.warnings.length">
        <li v-for="warning in proposal.warnings" :key="warning" class="warning-message">
          {{ warning }}
        </li>
      </ul>
      <label v-if="changed && proposal.warnings.length" class="acknowledge"
        ><input v-model="acknowledged" type="checkbox" /> I reviewed the warnings and
        assumptions</label
      >
      <div class="toolbar">
        <button
          v-if="changed"
          class="primary"
          :disabled="stale || (proposal.warnings.length > 0 && !acknowledged)"
          @click="apply"
        >
          Apply proposal</button
        ><button @click="discard">Discard proposal</button>
      </div>
    </section>
  </aside>
</template>
