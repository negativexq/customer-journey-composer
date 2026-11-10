<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount, nextTick } from 'vue';
import { api, type Journey } from '../lib/api';
import WorkflowCanvas from './WorkflowCanvas.vue';
import MessageStudio from './MessageStudio.vue';
import AIPanel from './AIPanel.vue';
import TestLab from './TestLab.vue';
import LiveMonitor from './LiveMonitor.vue';
import AnalyticsView from './AnalyticsView.vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import type { Workflow, Issue } from '../lib/workflow';
const props = defineProps<{
  journey: Journey;
  readonly?: boolean;
  admin?: boolean;
  activeTab?: string;
  previewSection?: string;
}>();
const emit = defineEmits<{
  back: [];
  saved: [Journey];
  navigate: [{ tab: string; preview: string }];
}>();
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
const tab = ref(props.activeTab || 'Builder');
const showAI = ref(draft.value.nodes.length === 0);
const metadata = ref<HTMLDetailsElement>();
const previewTab = ref(props.previewSection || 'Messages');
watch(
  () => [props.activeTab, props.previewSection],
  ([nextTab, nextPreview]) => {
    tab.value = nextTab || 'Builder';
    previewTab.value = nextPreview || 'Messages';
  },
);
watch([tab, previewTab], () => emit('navigate', { tab: tab.value, preview: previewTab.value }));

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
  const selected = draft.value.nodes.find((n) => n.id === id);
  previewTab.value =
    selected?.type === 'email' || selected?.type === 'sms' ? 'Messages' : 'Test Lab';
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
let saving: Promise<void> | undefined;
async function flushDraft() {
  if (saving) await saving;
  while (dirty.value) {
    await save();
    if (error.value) throw new Error(error.value);
  }
}
async function save() {
  if (saving) return saving;
  saving = saveDraft();
  try {
    await saving;
  } finally {
    saving = undefined;
  }
}
async function saveDraft() {
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
    await flushDraft();
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
  try {
    const gates = await api<{ ready: boolean; issues: { message: string }[]; policy: string }>(
      `/journeys/${props.journey.id}/publish-check`,
    );
    if (!gates.ready) {
      error.value = gates.issues.map((i) => i.message).join(' · ');
      return;
    }
    if (!window.confirm(gates.policy + ' Publish this version and allow new customers to enter?'))
      return;
  } catch (cause) {
    error.value = String(cause);
    return;
  }
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
async function saveTemplate() {
  const templateName = window.prompt('Template name', name.value);
  if (!templateName) return;
  try {
    await flushDraft();
    await api('/templates', 'POST', { journey_id: props.journey.id, name: templateName });
    notice.value = 'Workspace template saved';
  } catch (cause) {
    error.value = String(cause);
  }
}
async function lifecycle(action: string) {
  try {
    await flushDraft();
    const updated = await api<Journey>(`/journeys/${props.journey.id}/${action}`, 'POST', {
      revision: revision.value,
    });
    revision.value = updated.revision;
    emit('saved', updated);
    notice.value = 'Journey ' + updated.status.toLowerCase();
  } catch (cause) {
    error.value = String(cause);
  }
}
async function loadVersion(version: number) {
  if (versions.value.find((v) => v.version === version)?.workflow) return;
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
const beforeVersion = ref(1);
const afterVersion = ref(1);
const versionDiff = ref<unknown>();
async function compareVersions() {
  try {
    await Promise.all([...new Set([beforeVersion.value, afterVersion.value])].map(loadVersion));
    const before = versions.value.find((v) => v.version === beforeVersion.value)?.workflow;
    const after = versions.value.find((v) => v.version === afterVersion.value)?.workflow;
    if (!before || !after) return;
    const old = new Map(before.nodes.map((n) => [n.id, n]));
    const current = new Map(after.nodes.map((n) => [n.id, n]));
    versionDiff.value = {
      added_nodes: after.nodes.filter((n) => !old.has(n.id)).map((n) => n.label),
      removed_nodes: before.nodes.filter((n) => !current.has(n.id)).map((n) => n.label),
      updated_nodes: after.nodes
        .filter((n) => old.has(n.id) && JSON.stringify(old.get(n.id)) !== JSON.stringify(n))
        .map((n) => ({ node: n.label, before: old.get(n.id), after: n })),
      edges_changed: JSON.stringify(before.edges) !== JSON.stringify(after.edges),
      exit_changed: JSON.stringify(before.global_exit) !== JSON.stringify(after.global_exit),
      reentry_changed: JSON.stringify(before.reentry) !== JSON.stringify(after.reentry),
    };
  } catch (cause) {
    error.value = String(cause);
  }
}
async function showVersions() {
  try {
    versions.value = await api(`/journeys/${props.journey.id}/versions`);
    afterVersion.value = versions.value[0]?.version || 1;
    beforeVersion.value = versions.value.at(-1)?.version || 1;
  } catch (cause) {
    error.value = String(cause);
  }
}
watch(
  tab,
  (value) => {
    if (value === 'Versions') void showVersions();
  },
  { immediate: true },
);
function back() {
  if (dirty.value && !window.confirm('Discard unsaved changes?')) return;
  emit('back');
}
</script>
<template>
  <div class="journey-heading">
    <div class="journey-heading-copy">
      <button class="back-link" @click="back"><AppIcon name="back" :size="14" />Journeys</button>
      <div class="journey-title-row">
        <h1>{{ name }}</h1>
        <StatusBadge :status="journey.status" />
      </div>
      <p class="journey-subtitle">{{ description || 'Build a connected customer experience.' }}</p>
    </div>
    <div class="journey-heading-actions">
      <span class="save-feedback" role="status"
        ><AppIcon v-if="!dirty && !error" name="check" :size="14" />{{
          notice || 'All changes saved'
        }}</span
      >
      <div class="heading-actions">
        <button :disabled="busy || readonly || journey.status === 'Archived'" @click="save">
          Save</button
        ><button :disabled="busy" @click="validate">
          <AppIcon name="shield" :size="15" />Validate</button
        ><button
          @click="
            tab = 'Preview & Test';
            previewTab = 'Test Lab';
          "
        >
          <AppIcon name="flask" :size="15" />Test</button
        ><button
          v-if="admin && ['Active', 'Sunset'].includes(journey.status)"
          :disabled="busy"
          @click="lifecycle('pause')"
        >
          <AppIcon name="pause" :size="15" />Pause</button
        ><button
          v-if="admin && journey.status === 'Paused'"
          :disabled="busy"
          @click="lifecycle('resume')"
        >
          <AppIcon name="play" :size="15" />Resume</button
        ><button
          class="primary"
          :disabled="busy || readonly || !admin || journey.status === 'Archived'"
          @click="publish"
        >
          <AppIcon name="upload" :size="15" />Publish
        </button>
        <details class="action-menu">
          <summary aria-label="More journey actions"><AppIcon name="more" :size="19" /></summary>
          <div @click="($event.currentTarget as HTMLElement).closest('details')!.open = false">
            <button @click="metadata!.open = !metadata!.open">
              <AppIcon name="settings" />Journey settings</button
            ><button :disabled="readonly || busy" @click="saveTemplate">
              <AppIcon name="templates" />Save as template</button
            ><button
              v-if="admin && ['Active', 'Paused'].includes(journey.status)"
              :disabled="busy"
              @click="lifecycle('sunset')"
            >
              Sunset</button
            ><button
              v-if="admin && ['Sunset', 'Paused'].includes(journey.status)"
              :disabled="busy"
              @click="lifecycle('complete')"
            >
              Complete after draining</button
            ><button
              v-if="admin && ['Draft', 'Completed'].includes(journey.status)"
              :disabled="busy"
              @click="lifecycle('archive')"
            >
              Archive
            </button>
          </div>
        </details>
      </div>
    </div>
  </div>
  <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
  <details ref="metadata" class="journey-settings">
    <summary>
      <span><AppIcon name="settings" :size="15" />Journey settings</span
      ><span class="journey-settings-summary"
        >{{ exitEvent ? 'Exit on ' + exitEvent : 'No global exit' }} ·
        {{ reentryMode === 'once' ? 'Enter once' : 'Re-entry enabled'
        }}<AppIcon name="down" :size="14"
      /></span>
    </summary>
    <section class="editor-form journey-settings-form">
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
  </details>
  <nav class="detail-tabs journey-tabs">
    <button
      v-for="[label, icon] in [
        ['Builder', 'workflow'],
        ['Preview & Test', 'flask'],
        ['Live Monitor', 'events'],
        ['Analytics', 'chart'],
        ['Versions', 'history'],
      ]"
      :key="label"
      :class="{ selected: tab === label }"
      @click="tab = label"
    >
      <AppIcon :name="icon" :size="16" />{{ label }}</button
    ><button
      v-if="tab === 'Builder'"
      class="ai-toggle"
      :class="{ selected: showAI }"
      @click="showAI = !showAI"
    >
      <AppIcon name="sparkles" :size="16" />AI composer
    </button>
  </nav>
  <section v-if="tab === 'Builder'" class="composer-layout" :class="{ 'ai-open': showAI }">
    <div class="builder-column">
      <WorkflowCanvas
        :key="canvasKey"
        v-model="draft"
        :readonly="readonly || journey.status === 'Archived'"
        :issues="issues"
        :hide-inspector="showAI"
        @inspect="showAI = false"
        @preview="previewNode"
      />
    </div>
    <AIPanel
      v-show="showAI"
      :workflow="draft"
      :name="name"
      :description="description"
      :readonly="readonly || journey.status === 'Archived'"
      @apply="applyProposal"
    />
  </section>
  <section v-else-if="tab === 'Preview & Test'">
    <nav class="detail-tabs">
      <button @click="previewTab = 'Messages'">Message preview</button
      ><button @click="previewTab = 'Test Lab'">Journey path &amp; Test Lab</button>
    </nav>
    <TestLab
      v-if="previewTab === 'Test Lab'"
      :journey-id="journey.id"
      :before-run="flushDraft"
      :readonly="readonly"
      :node-id="messageNodeID"
    />
    <template v-else>
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
    </template>
  </section>
  <LiveMonitor
    v-else-if="tab === 'Live Monitor'"
    :journey-id="journey.id"
    :readonly="readonly"
    :admin="admin"
  /><AnalyticsView v-else-if="tab === 'Analytics'" :journey-id="journey.id" />
  <div v-else class="panel">
    <h2>Published versions</h2>
    <div v-if="versions.length > 1" class="editor-form">
      <label
        >Compare from<select v-model.number="beforeVersion">
          <option v-for="v in versions" :key="v.id" :value="v.version">v{{ v.version }}</option>
        </select></label
      ><label
        >Compare to<select v-model.number="afterVersion">
          <option v-for="v in versions" :key="v.id" :value="v.version">v{{ v.version }}</option>
        </select></label
      ><button @click="compareVersions">Compare versions</button>
    </div>
    <pre v-if="versionDiff">{{ JSON.stringify(versionDiff, null, 2) }}</pre>
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
