<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount, nextTick } from 'vue';
import { api, type Journey } from '../lib/api';
import WorkflowCanvas from './WorkflowCanvas.vue';
import MessageStudio from './MessageStudio.vue';
import AIPanel from './AIPanel.vue';
import type { Workflow, Issue } from '../lib/workflow';
const props = defineProps<{ journey: Journey; readonly?: boolean }>();
const emit = defineEmits<{ back: []; saved: [Journey] }>();
const name = ref(props.journey.name);
const description = ref(props.journey.description);
const tags = ref(props.journey.tags.join(', '));
const draft = ref(props.journey.draft as unknown as Workflow);
const issues = ref<Issue[]>([]);
const canvasKey = ref(0);
let applying = false;
const exitEvent = ref(draft.value.global_exit?.event_name || '');
const reentryMode = ref(draft.value.reentry?.mode || 'after_completion');
const cooldown = ref(draft.value.reentry?.cooldown_seconds || 86400);
watch([exitEvent, reentryMode, cooldown], () => {
  if (applying) return;
  draft.value.global_exit = exitEvent.value ? { event_name: exitEvent.value } : undefined;
  draft.value.reentry = {
    mode: reentryMode.value,
    ...(reentryMode.value === 'cooldown' ? { cooldown_seconds: cooldown.value } : {}),
  };
});
const tab = ref('Builder');
const messageNodeID = ref('');
const messageStep = computed(
  () =>
    draft.value.nodes.find((n) => n.id === messageNodeID.value) ||
    draft.value.nodes.find((n) => n.type === 'email' || n.type === 'sms'),
);
async function applyProposal(proposal: { workflow: Workflow; name: string; description: string }) {
  applying = true;
  draft.value = JSON.parse(JSON.stringify(proposal.workflow));
  name.value = proposal.name;
  description.value = proposal.description;
  exitEvent.value = draft.value.global_exit?.event_name || '';
  reentryMode.value = draft.value.reentry?.mode || 'after_completion';
  cooldown.value = draft.value.reentry?.cooldown_seconds || 86400;
  issues.value = [];
  canvasKey.value++;
  await nextTick();
  applying = false;
}
function previewNode(id: string) {
  messageNodeID.value = id;
  tab.value = 'Preview & Test';
}
const versions = ref<{ id: string; version: number; workflow?: Workflow; created_at: string }[]>(
  [],
);
let autosave: ReturnType<typeof setTimeout> | undefined;
const revision = ref(props.journey.revision);
const dirty = ref(false);
const busy = ref(false);
const notice = ref('');
const error = ref('');
watch(
  [name, description, tags, draft],
  () => {
    dirty.value = true;
    notice.value = 'Unsaved changes';
    clearTimeout(autosave);
    if (!props.readonly && props.journey.status !== 'Archived')
      autosave = setTimeout(() => {
        if (!busy.value) void save();
      }, 1500);
  },
  { deep: true },
);
function warn(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
  }
}
window.addEventListener('beforeunload', warn);
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', warn);
  clearTimeout(autosave);
});
async function save() {
  if (busy.value) return;
  clearTimeout(autosave);
  busy.value = true;
  error.value = '';
  try {
    const snapshot = JSON.stringify([name.value, description.value, tags.value, draft.value]);
    const journey = await api<Journey>(`/journeys/${props.journey.id}/draft`, 'PUT', {
      name: name.value,
      description: description.value,
      tags: tags.value
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
      draft: draft.value,
      revision: revision.value,
    });
    revision.value = journey.revision;
    dirty.value =
      snapshot !== JSON.stringify([name.value, description.value, tags.value, draft.value]);
    notice.value = dirty.value ? 'Unsaved changes' : 'All changes saved';
    if (dirty.value) autosave = setTimeout(() => void save(), 500);
    emit('saved', journey);
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
async function validate() {
  error.value = '';
  try {
    if (dirty.value) await save();
    const result = await api<{ valid: boolean; issues: Issue[] }>(
      `/journeys/${props.journey.id}/validate`,
      'POST',
    );
    issues.value = result.issues;
    notice.value = result.valid ? 'Workflow is valid' : `${result.issues.length} validation issues`;
  } catch (cause) {
    error.value = String(cause);
  }
}
async function publish() {
  await validate();
  if (issues.value.length || error.value) return;
  if (!window.confirm('Publish this version and allow new customers to enter?')) return;
  try {
    await api(`/journeys/${props.journey.id}/publish`, 'POST', { revision: revision.value });
    const updated = await api<Journey>(`/journeys/${props.journey.id}`);
    revision.value = updated.revision;
    emit('saved', updated);
    notice.value = 'Journey published';
  } catch (cause) {
    error.value = String(cause);
  }
}
async function loadVersion(version: number) {
  try {
    const result = await api<{ workflow: Workflow }>(
      `/journeys/${props.journey.id}/versions/${version}`,
    );
    const target = versions.value.find((v) => v.version === version);
    if (target) target.workflow = result.workflow;
  } catch (cause) {
    error.value = String(cause);
  }
}
async function showVersions() {
  tab.value = 'Versions';
  try {
    versions.value = await api(`/journeys/${props.journey.id}/versions`);
  } catch (cause) {
    error.value = String(cause);
  }
}
function back() {
  if (dirty.value && !window.confirm('Discard unsaved changes?')) return;
  emit('back');
}
</script>
<template>
  <div class="page-heading">
    <div>
      <button @click="back">← Journeys</button>
      <h1>{{ name }}</h1>
      <p class="page-description">{{ journey.status }} · Revision {{ revision }}</p>
    </div>
    <div>
      <button
        class="primary"
        :disabled="busy || readonly || journey.status === 'Archived'"
        @click="save"
      >
        Save
      </button>
      <button :disabled="busy" @click="validate">Validate</button
      ><button :disabled="busy || readonly" @click="publish">Publish</button>
      <p role="status">{{ notice }}</p>
    </div>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <section class="panel editor-form">
    <label>Name<input v-model="name" :disabled="readonly" maxlength="160" required /></label
    ><label
      >Description<textarea v-model="description" :disabled="readonly" maxlength="4000" /></label
    ><label
      >Tags<input v-model="tags" :disabled="readonly" placeholder="Comma-separated tags"
    /></label>
    <label
      >Global exit event<input
        v-model="exitEvent"
        :disabled="readonly"
        placeholder="e.g. purchase_completed"
    /></label>
    <label
      >Re-entry policy<select v-model="reentryMode" :disabled="readonly">
        <option value="once">Only once</option>
        <option value="after_completion">After previous execution ends</option>
        <option value="cooldown">After cooldown</option>
      </select></label
    ><label v-if="reentryMode === 'cooldown'"
      >Cooldown (seconds)<input
        v-model.number="cooldown"
        type="number"
        min="1"
        :disabled="readonly"
    /></label>
  </section>
  <nav class="detail-tabs">
    <button :class="{ selected: tab === 'Builder' }" @click="tab = 'Builder'">Builder</button
    ><button @click="tab = 'Preview & Test'">Preview &amp; Test</button
    ><button @click="showVersions">Versions</button>
  </nav>
  <section v-if="tab === 'Builder'" class="composer-layout">
    <div class="builder-column">
      <WorkflowCanvas
        :key="canvasKey"
        v-model="draft"
        :readonly="readonly || journey.status === 'Archived'"
        :issues="issues"
        @preview="previewNode"
      />
    </div>
    <AIPanel
      :workflow="draft"
      :name="name"
      :description="description"
      :readonly="readonly || journey.status === 'Archived'"
      @apply="applyProposal"
    />
  </section>
  <section v-else-if="tab === 'Preview & Test'">
    <label
      >Message node<select v-model="messageNodeID">
        <option value="">First message</option>
        <option
          v-for="step in draft.nodes.filter((n) => n.type === 'email' || n.type === 'sms')"
          :key="step.id"
          :value="step.id"
        >
          {{ step.label }}
        </option>
      </select></label
    ><MessageStudio
      v-if="messageStep"
      :key="messageStep.id"
      v-model="messageStep.config"
      :channel="messageStep.type === 'email' ? 'email' : 'sms'"
      :readonly="readonly"
    />
    <p v-else class="panel">Add an email or SMS node in Builder to preview a message.</p>
  </section>
  <div v-else class="panel">
    <h2>Published versions</h2>
    <p v-if="!versions.length">No published versions yet.</p>
    <details v-for="version in versions" :key="version.id">
      <summary>
        Version {{ version.version }} · {{ new Date(version.created_at).toLocaleString() }}
      </summary>
      <button v-if="!version.workflow" @click="loadVersion(version.version)">Load workflow</button>
      <pre v-else>{{ JSON.stringify(version.workflow, null, 2) }}</pre>
    </details>
  </div>
  <section v-if="issues.length" class="panel">
    <h3>Validation</h3>
    <p v-for="(issue, index) in issues" :key="index" class="error-message">
      {{ issue.node_id || 'Workflow' }}: {{ issue.message }}
    </p>
  </section>
</template>
