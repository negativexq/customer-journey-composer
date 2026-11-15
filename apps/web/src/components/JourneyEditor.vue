<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import EmptyState from './ui/EmptyState.vue';
import { toast } from '../lib/toast';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { computed, ref, watch, onBeforeUnmount, onMounted, nextTick } from 'vue';
import { api, type Journey, type GenerationProgress } from '../lib/api';
import WorkflowCanvas from './WorkflowCanvas.vue';
import MessageStudio from './MessageStudio.vue';
import AIPanel from './AIPanel.vue';
import GenerationStatus from './GenerationStatus.vue';
import PublishReview from './PublishReview.vue';
import TestLab from './TestLab.vue';
import LiveMonitor from './LiveMonitor.vue';
import AnalyticsView from './AnalyticsView.vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import ScopeSelect from './ui/ScopeSelect.vue';
import FilterSelect from './ui/FilterSelect.vue';
import BehaviorChecksEditor from './BehaviorChecksEditor.vue';
import { blankBehaviorChecks } from '../lib/behavior-checks';
import type { DataEntry } from '../lib/data-catalog';
import { labels, type Workflow, type Issue } from '../lib/workflow';
import { stepSummary } from '../lib/workflow-presentation';
import { diffWorkflows, type WorkflowDiff } from '../lib/version-diff';
import ConfirmDialog from './ui/ConfirmDialog.vue';
import JourneyDiagram from './JourneyDiagram.vue';
import WorkflowChanges from './WorkflowChanges.vue';
import type { InspectionKind } from '../lib/navigation';
const props = defineProps<{
  journey: Journey;
  readonly?: boolean;
  admin?: boolean;
  activeTab?: string;
  previewSection?: string;
  activeExecutionId?: string;
  activeDeliveryId?: string;
}>();
const emit = defineEmits<{
  back: [];
  settings: [];
  saved: [Journey];
  navigate: [{ tab: string; preview: string; executionID?: string; deliveryID?: string }];
  inspect: [{ kind: InspectionKind; id: string }];
}>();
const aiPanel = ref<InstanceType<typeof AIPanel>>();
const generation = ref<{ progress: GenerationProgress; elapsed: number }>();
const name = ref(props.journey.name);
const description = ref(props.journey.description);
const tags = ref(props.journey.tags.join(', '));
const draft = ref(props.journey.draft as unknown as Workflow);
const issues = ref<Issue[]>([]);
const canvasKey = ref(0);
const publishedVersion = ref(props.journey.version);
watch(
  () => props.journey.version,
  (value) => {
    if (value != null) publishedVersion.value = value;
  },
);
let applying = false;
const exitEvent = ref(draft.value.global_exit?.event_name || '');
const exitReason = ref<NonNullable<Workflow['global_exit']>['reason']>(
  draft.value.global_exit?.reason || 'converted',
);
const exitScope = ref(draft.value.global_exit?.correlation ? 'entry' : 'customer');
const exitEntryPath = ref(draft.value.global_exit?.correlation?.entry_path || '');
const exitIncomingPath = ref(draft.value.global_exit?.correlation?.event_path || '');
const exitFields = ref<DataEntry[]>([]);
const exitFieldsError = ref('');
let exitFieldsLoaded = false;
async function loadExitFields() {
  if (!metadata.value?.open || exitFieldsLoaded) return;
  try {
    exitFields.value = await api('/data-catalog');
    exitFieldsLoaded = true;
    exitFieldsError.value = '';
  } catch {
    exitFieldsError.value =
      'Workspace fields could not be loaded. Close and reopen settings to retry.';
  }
}
function correlationOptions(eventName: string, saved: string) {
  const options = exitFields.value
    .filter(
      (e) =>
        e.enabled &&
        e.kind === 'event_field' &&
        e.event_name === eventName &&
        ['string', 'number'].includes(e.data_type),
    )
    .map((e) => ({ value: e.path!, label: e.label }));
  if (saved && !options.some((o) => o.value === saved))
    options.push({ value: saved, label: 'Saved field: ' + saved });
  return [{ value: 'all', label: 'Choose an identifier' }, ...options];
}
const entryIdentityOptions = computed(() =>
  correlationOptions(
    draft.value.nodes.find((n) => n.type === 'event_trigger')?.config.event_name || '',
    exitEntryPath.value,
  ),
);
const incomingIdentityOptions = computed(() =>
  correlationOptions(exitEvent.value, exitIncomingPath.value),
);
const reentryMode = ref(draft.value.reentry?.mode || 'after_completion');
const cooldown = ref(draft.value.reentry?.cooldown_seconds || 86400);
watch(
  [exitEvent, exitReason, reentryMode, cooldown, exitScope, exitEntryPath, exitIncomingPath],
  () => {
    if (applying) return;
    draft.value.global_exit = exitEvent.value
      ? {
          ...draft.value.global_exit,
          event_name: exitEvent.value,
          reason: exitReason.value,
          correlation:
            exitScope.value === 'entry'
              ? { entry_path: exitEntryPath.value, event_path: exitIncomingPath.value }
              : undefined,
        }
      : undefined;
    draft.value.reentry = {
      mode: reentryMode.value,
      ...(reentryMode.value === 'cooldown' ? { cooldown_seconds: cooldown.value } : {}),
    };
  },
);
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
const testNodeID = ref('');
const messageStep = computed(
  () =>
    draft.value.nodes.find((n) => n.id === messageNodeID.value) ||
    draft.value.nodes.find((n) => n.type === 'email' || n.type === 'sms'),
);
const messageOptions = computed(() =>
  draft.value.nodes
    .filter((n) => n.type === 'email' || n.type === 'sms')
    .map((n, i) => ({
      value: n.id,
      label: n.label,
      description: (n.type === 'email' ? 'Email' : 'SMS') + ' · message ' + (i + 1),
    })),
);
async function applyProposal(proposal: { workflow: Workflow; name: string; description: string }) {
  applying = true;
  draft.value = JSON.parse(JSON.stringify(proposal.workflow));
  name.value = proposal.name;
  description.value = proposal.description;
  exitEvent.value = draft.value.global_exit?.event_name || '';
  exitReason.value = draft.value.global_exit?.reason || 'converted';
  exitScope.value = draft.value.global_exit?.correlation ? 'entry' : 'customer';
  exitEntryPath.value = draft.value.global_exit?.correlation?.entry_path || '';
  exitIncomingPath.value = draft.value.global_exit?.correlation?.event_path || '';
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
  if (previewTab.value === 'Test Lab') testNodeID.value = id;
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
    error.value = errorMessage(cause);
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
    error.value = errorMessage(cause);
  }
}
const publishOpen = ref(false);
const publishRevision = ref(revision.value);
const publishError = ref('');
async function publish() {
  error.value = '';
  publishError.value = '';
  try {
    await flushDraft();
    publishRevision.value = revision.value;
    // The review states what this publish changes for new entries.
    if (!versions.value.length)
      versions.value = await api<VersionRow[]>(`/journeys/${props.journey.id}/versions`);
    await compareDraft();
    publishOpen.value = true;
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function confirmPublish() {
  if (busy.value) return;
  busy.value = true;
  publishError.value = '';
  try {
    await api(`/journeys/${props.journey.id}/publish`, 'POST', { revision: publishRevision.value });
    const updated = await api<Journey>(`/journeys/${props.journey.id}`);
    revision.value = updated.revision;
    emit('saved', updated);
    publishOpen.value = false;
    // A new version exists now; the next comparison must load it.
    versions.value = [];
    draftDiff.value = undefined;
    notice.value = 'Journey published';
    toast('Journey published', { announce: false });
  } catch (cause) {
    publishError.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
function fixPublishIssue(code: string) {
  if (code.startsWith('test_')) {
    tab.value = 'Preview & Test';
    previewTab.value = 'Test Lab';
  } else if (code === 'email_provider') emit('settings');
  else {
    tab.value = 'Builder';
    void validate();
  }
}
async function saveTemplate() {
  const templateName = window.prompt('Template name', name.value);
  if (!templateName) return;
  try {
    await flushDraft();
    await api('/templates', 'POST', { journey_id: props.journey.id, name: templateName });
    notice.value = 'Workspace template saved';
    toast('Workspace template saved', { announce: false });
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
// The overflow menu is a native <details>; close it like a menu on outside press or Escape.
const actionMenu = ref<HTMLDetailsElement>();
function closeActionMenu(restoreFocus = false) {
  if (!actionMenu.value?.open) return;
  actionMenu.value.open = false;
  if (restoreFocus) actionMenu.value.querySelector('summary')?.focus();
}
function outsideActionMenu(event: PointerEvent) {
  if (actionMenu.value?.open && !actionMenu.value.contains(event.target as Node)) closeActionMenu();
}
onMounted(() => document.addEventListener('pointerdown', outsideActionMenu));
onBeforeUnmount(() => document.removeEventListener('pointerdown', outsideActionMenu));
type GuardedAction = 'sunset' | 'complete' | 'archive';
const pendingAction = ref<GuardedAction>();
const lifecycleBusy = ref(false);
const confirmCopy: Record<GuardedAction, { title: string; label: string; danger?: boolean }> = {
  sunset: { title: 'Sunset this journey?', label: 'Sunset journey' },
  complete: { title: 'Mark this journey completed?', label: 'Complete journey' },
  archive: { title: 'Archive this journey?', label: 'Archive journey', danger: true },
};
async function confirmLifecycle() {
  const action = pendingAction.value;
  if (!action || lifecycleBusy.value) return;
  lifecycleBusy.value = true;
  try {
    await lifecycle(action);
  } finally {
    lifecycleBusy.value = false;
    pendingAction.value = undefined;
  }
}
async function lifecycle(action: string) {
  error.value = '';
  try {
    await flushDraft();
    const updated = await api<Journey>(`/journeys/${props.journey.id}/${action}`, 'POST', {
      revision: revision.value,
    });
    revision.value = updated.revision;
    emit('saved', updated);
    notice.value = 'Journey ' + updated.status.toLowerCase();
    toast(notice.value, { announce: false });
  } catch (cause) {
    error.value = errorMessage(cause);
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
    error.value = errorMessage(cause);
  }
}
type VersionRow = { id: string; version: number; workflow?: Workflow; created_at: string };
const versionsLoading = ref(false);
const openVersion = ref<number>();
const showDefinition = ref(false);
const beforeVersion = ref(1);
const afterVersion = ref(1);
const versionDiff = ref<{ from: number; to: number; diff: WorkflowDiff }>();
const draftDiff = ref<WorkflowDiff>();
const compare = (before: Workflow, after: Workflow) =>
  diffWorkflows(before, after, stepSummary, (step) => labels[step.type]);
const versionWorkflow = (version: number) =>
  versions.value.find((v) => v.version === version)?.workflow;
async function toggleVersion(version: number) {
  showDefinition.value = false;
  if (openVersion.value === version) {
    openVersion.value = undefined;
    return;
  }
  openVersion.value = version;
  await loadVersion(version);
}
async function compareVersions() {
  versionDiff.value = undefined;
  await Promise.all([...new Set([beforeVersion.value, afterVersion.value])].map(loadVersion));
  const before = versionWorkflow(beforeVersion.value);
  const after = versionWorkflow(afterVersion.value);
  if (!before || !after) return;
  versionDiff.value = {
    from: beforeVersion.value,
    to: afterVersion.value,
    diff: compare(before, after),
  };
}
// What publishing now would change for new entries, compared with the latest version.
async function compareDraft() {
  const latest = versions.value[0];
  draftDiff.value = undefined;
  if (!latest) return;
  await loadVersion(latest.version);
  const published = versionWorkflow(latest.version);
  if (published) draftDiff.value = compare(published, draft.value);
}
// Puts a published version back on the draft. Nothing customers see changes until it is published.
const restoring = ref<VersionRow>();
const restoreBusy = ref(false);
async function confirmRestore() {
  const version = restoring.value;
  if (!version?.workflow || restoreBusy.value) return;
  restoreBusy.value = true;
  error.value = '';
  try {
    await applyProposal({
      workflow: version.workflow,
      name: name.value,
      description: description.value,
    });
    await flushDraft();
    await compareDraft();
    toast(`Version ${version.version} restored to the draft`);
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    restoreBusy.value = false;
    restoring.value = undefined;
  }
}
async function showVersions() {
  versionsLoading.value = true;
  try {
    versions.value = await api<VersionRow[]>(`/journeys/${props.journey.id}/versions`);
    afterVersion.value = versions.value[0]?.version || 1;
    beforeVersion.value = versions.value[1]?.version || versions.value[0]?.version || 1;
    versionDiff.value = undefined;
    await compareDraft();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    versionsLoading.value = false;
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
      <div class="journey-title-row">
        <button
          class="back-link"
          type="button"
          aria-label="Back to journeys"
          title="Back to journeys"
          @click="back"
        >
          <AppIcon name="back" :size="16" />
        </button>
        <h1 :title="name">{{ name }}</h1>
        <StatusBadge :status="journey.status" />
      </div>
      <div class="draft-context">
        <strong>{{
          ['Live Monitor', 'Analytics'].includes(tab)
            ? 'Published journey'
            : readonly
              ? 'Viewing draft'
              : 'Editing draft'
        }}</strong
        ><span class="version-context"
          ><AppIcon name="history" :size="13" />{{
            publishedVersion ? 'Latest published v' + publishedVersion : 'Not published yet'
          }}</span
        ><span
          v-if="tab !== 'Builder' && description"
          class="journey-subtitle"
          :title="description"
          >{{ description }}</span
        >
      </div>
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
            testNodeID = '';
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
        <details ref="actionMenu" class="action-menu" @keydown.esc="closeActionMenu(true)">
          <summary aria-label="More journey actions"><AppIcon name="more" :size="19" /></summary>
          <div @click="($event.currentTarget as HTMLElement).closest('details')!.open = false">
            <button @click="metadata!.open = !metadata!.open">
              <AppIcon name="settings" />Journey settings</button
            ><button :disabled="readonly || busy" @click="saveTemplate">
              <AppIcon name="templates" />Save as template</button
            ><button
              v-if="admin && ['Active', 'Paused'].includes(journey.status)"
              :disabled="busy"
              @click="pendingAction = 'sunset'"
            >
              <AppIcon name="exit" />Sunset</button
            ><button
              v-if="admin && ['Sunset', 'Paused'].includes(journey.status)"
              :disabled="busy"
              @click="pendingAction = 'complete'"
            >
              <AppIcon name="check" />Complete after draining</button
            ><button
              v-if="admin && ['Draft', 'Completed'].includes(journey.status)"
              :disabled="busy"
              @click="pendingAction = 'archive'"
            >
              <AppIcon name="folder" />Archive
            </button>
          </div>
        </details>
      </div>
    </div>
  </div>
  <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
  <details ref="metadata" class="journey-settings" @toggle="loadExitFields">
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
        >Global exit outcome<AppSelect
          v-model="exitReason"
          :disabled="readonly || !exitEvent"
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
      <ScopeSelect
        v-model="exitScope"
        label="Exit matching"
        :disabled="readonly || !exitEvent"
        :options="[
          { value: 'customer', label: 'Any matching event from this customer' },
          { value: 'entry', label: 'Only the object that started this journey' },
        ]"
      />
      <template v-if="exitScope === 'entry' && exitEvent">
        <p>
          Match the incoming event identifier to the identifier recorded at entry. Missing or
          different identifiers do not end the journey.
        </p>
        <ScopeSelect
          :model-value="exitEntryPath || 'all'"
          label="Entry identifier"
          :options="entryIdentityOptions"
          :disabled="readonly"
          @update:model-value="exitEntryPath = $event === 'all' ? '' : $event"
        />
        <ScopeSelect
          :model-value="exitIncomingPath || 'all'"
          label="Incoming identifier"
          :options="incomingIdentityOptions"
          :disabled="readonly"
          @update:model-value="exitIncomingPath = $event === 'all' ? '' : $event"
        />
        <p v-if="exitFieldsError" role="alert">{{ exitFieldsError }}</p>
        <p v-else-if="entryIdentityOptions.length === 1 || incomingIdentityOptions.length === 1">
          Define identifier fields in Settings → Workspace data.
        </p>
      </template>
      <SegmentedControl
        v-model="reentryMode"
        label="Customers can enter again"
        :disabled="readonly"
        :options="[
          { value: 'once', label: 'Never' },
          { value: 'after_completion', label: 'After they finish' },
          { value: 'cooldown', label: 'After a cooldown' },
        ]"
      /><label v-if="reentryMode === 'cooldown'"
        >Cooldown (seconds)<input
          v-model.number="cooldown"
          type="number"
          min="1"
          :disabled="readonly"
      /></label>
    </section>
    <BehaviorChecksEditor
      v-if="draft.behavior_checks"
      v-model="draft.behavior_checks"
      :disabled="readonly"
    />
    <button
      v-else-if="!readonly"
      type="button"
      @click="draft.behavior_checks = blankBehaviorChecks()"
    >
      Add independent behavior checks
    </button>
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
      <AppIcon :name="icon" :size="16" />{{ label }}
    </button>
  </nav>
  <section v-if="tab === 'Builder'" class="composer-layout" :class="{ 'ai-open': showAI }">
    <div class="builder-column" :aria-busy="!!generation">
      <div v-if="generation" class="canvas-generation" :class="{ empty: !draft.nodes.length }">
        <GenerationStatus
          :progress="generation.progress"
          :elapsed="generation.elapsed"
          compact
          @cancel="aiPanel?.cancel()"
        />
      </div>
      <WorkflowCanvas
        :key="canvasKey"
        v-model="draft"
        :readonly="readonly || journey.status === 'Archived'"
        :issues="issues"
        :hide-inspector="showAI"
        :ai-open="showAI"
        @inspect="showAI = false"
        @preview="previewNode"
        @ai="showAI = !showAI"
      />
    </div>
    <AIPanel
      v-show="showAI"
      ref="aiPanel"
      :workflow="draft"
      :name="name"
      :description="description"
      :readonly="readonly || journey.status === 'Archived'"
      @progress="generation = $event"
      @apply="applyProposal"
    />
  </section>
  <section v-else-if="tab === 'Preview & Test'">
    <nav class="preview-switch" aria-label="Preview views">
      <button
        type="button"
        :aria-pressed="previewTab !== 'Test Lab'"
        @click="previewTab = 'Messages'"
      >
        <AppIcon name="email" :size="15" />Message preview</button
      ><button
        type="button"
        :aria-pressed="previewTab === 'Test Lab'"
        @click="
          previewTab = 'Test Lab';
          testNodeID = '';
        "
      >
        <AppIcon name="flask" :size="15" />Journey path &amp; Test Lab
      </button>
    </nav>
    <template v-if="previewTab !== 'Test Lab'">
      <FilterSelect
        class="message-node-select"
        label="Message step"
        icon="email"
        :model-value="messageStep?.id || ''"
        :options="messageOptions"
        @update:model-value="messageNodeID = $event"
      /><MessageStudio
        v-if="messageStep"
        :key="messageStep.id"
        v-model="messageStep.config"
        :channel="messageStep.type === 'email' ? 'email' : 'sms'"
        :readonly="readonly"
      />
      <p v-else class="panel">Add an email or SMS node in Builder to preview a message.</p>
    </template>
  </section>
  <AnalyticsView
    v-else-if="tab === 'Analytics'"
    :journey-id="journey.id"
    :initial-execution-id="activeExecutionId"
    :initial-delivery-id="activeDeliveryId"
    @inspect="emit('inspect', $event)"
    @selection="emit('navigate', { tab, preview: previewTab, ...$event })"
  />
  <section v-else-if="tab === 'Versions'" class="versions-view">
    <div class="versions-heading">
      <div>
        <h2>Published versions</h2>
        <p>
          Each publish creates an immutable version. Customers already in the journey keep the
          version they entered with.
        </p>
      </div>
      <span v-if="versions.length" class="count-label">{{
        plural(versions.length, 'version')
      }}</span>
    </div>
    <SkeletonBlock v-if="versionsLoading && !versions.length" label="Loading versions…" :rows="2" />
    <EmptyState v-else-if="!versions.length" icon="history" title="No published versions yet">
      Publish the draft to create version 1. Until then no customer can enter.
    </EmptyState>
    <template v-else>
      <div
        v-if="draftDiff"
        class="draft-changes"
        :class="{ 'has-changes': draftDiff.total > 0 }"
        data-testid="draft-changes"
      >
        <div class="draft-changes-title">
          <AppIcon :name="draftDiff.total ? 'file' : 'check'" :size="16" />
          <strong>{{
            draftDiff.total
              ? `Draft has ${plural(draftDiff.total, 'unpublished change')} since v${versions[0]!.version}`
              : `Draft matches v${versions[0]!.version}`
          }}</strong>
        </div>
        <p v-if="!draftDiff.total">Publishing now would not change journey behavior.</p>
        <WorkflowChanges v-else :diff="draftDiff" />
      </div>
      <form v-if="versions.length > 1" class="version-compare" @submit.prevent="compareVersions">
        <label
          >Compare from<AppSelect
            v-model="beforeVersion"
            :options="versions.map((v) => ({ value: v.version, label: 'v' + v.version }))" /></label
        ><label
          >Compare to<AppSelect
            v-model="afterVersion"
            :options="versions.map((v) => ({ value: v.version, label: 'v' + v.version }))" /></label
        ><button type="submit" :disabled="beforeVersion === afterVersion">Compare versions</button>
      </form>
      <div v-if="versionDiff" class="draft-changes has-changes" data-testid="version-changes">
        <div class="draft-changes-title">
          <AppIcon name="history" :size="16" />
          <strong
            >v{{ versionDiff.from }} → v{{ versionDiff.to }} ·
            {{ plural(versionDiff.diff.total, 'change') }}</strong
          >
        </div>
        <p v-if="!versionDiff.diff.total">These versions behave the same.</p>
        <WorkflowChanges v-else :diff="versionDiff.diff" />
      </div>
      <ol class="version-list">
        <li v-for="(version, index) in versions" :key="version.id" class="version-card">
          <button
            type="button"
            class="version-row"
            :aria-expanded="openVersion === version.version"
            @click="toggleVersion(version.version)"
          >
            <span class="version-badge">v{{ version.version }}</span>
            <span class="version-row-copy"
              ><strong>Version {{ version.version }}</strong
              ><small>Published {{ formatDateTime(version.created_at) }}</small></span
            >
            <span v-if="index === 0" class="status-badge tone-green"
              ><i />{{
                journey.status === 'Active' ? 'Receiving new entries' : 'Latest version'
              }}</span
            >
            <AppIcon
              name="down"
              :size="15"
              class="version-chevron"
              :class="{ open: openVersion === version.version }"
            />
          </button>
          <div v-if="openVersion === version.version" class="version-detail">
            <p v-if="!version.workflow" role="status">Loading version {{ version.version }}…</p>
            <template v-else>
              <JourneyDiagram :workflow="version.workflow" />
              <button type="button" class="text-button" @click="showDefinition = !showDefinition">
                {{ showDefinition ? 'Hide technical definition' : 'Show technical definition' }}
              </button>
              <button
                v-if="!readonly && journey.status !== 'Archived'"
                type="button"
                class="restore-version"
                @click="restoring = version"
              >
                <AppIcon name="history" :size="15" />Restore to draft
              </button>
              <pre v-if="showDefinition">{{ JSON.stringify(version.workflow, null, 2) }}</pre>
            </template>
          </div>
        </li>
      </ol>
    </template>
  </section>
  <KeepAlive
    ><TestLab
      v-if="tab === 'Preview & Test' && previewTab === 'Test Lab'"
      :journey-id="journey.id"
      :workflow="draft"
      :journey-tags="journey.tags"
      :before-run="flushDraft"
      :readonly="readonly"
      :node-id="testNodeID"
  /></KeepAlive>
  <KeepAlive
    ><LiveMonitor
      v-if="tab === 'Live Monitor'"
      :journey-id="journey.id"
      :readonly="readonly"
      :admin="admin"
      :initial-execution-id="activeExecutionId"
      :initial-delivery-id="activeDeliveryId"
      @selection="emit('navigate', { tab: tab, preview: previewTab, ...$event })"
      @inspect="emit('inspect', $event)"
  /></KeepAlive>
  <ConfirmDialog
    :model-value="!!pendingAction"
    :title="pendingAction ? confirmCopy[pendingAction].title : ''"
    :confirm-label="pendingAction ? confirmCopy[pendingAction].label : 'Confirm'"
    :danger="pendingAction ? confirmCopy[pendingAction].danger : false"
    :busy="lifecycleBusy"
    @update:model-value="pendingAction = undefined"
    @confirm="confirmLifecycle"
  >
    <template v-if="pendingAction === 'sunset'">
      <p>
        <strong>{{ name }}</strong> stops accepting new entries.
      </p>
      <p>Customers already in the journey continue to the end.</p>
    </template>
    <template v-else-if="pendingAction === 'complete'">
      <p>
        <strong>{{ name }}</strong> is marked completed and stays closed to new entries.
      </p>
      <p>Its versions, history and analytics remain available.</p>
    </template>
    <template v-else-if="pendingAction === 'archive'">
      <p>
        <strong>{{ name }}</strong> moves to the Archived list and can no longer be edited or
        published.
      </p>
      <p>Its history is kept. There is no restore action.</p>
    </template>
  </ConfirmDialog>
  <ConfirmDialog
    :model-value="!!restoring"
    :title="`Restore version ${restoring?.version} to the draft?`"
    confirm-label="Restore to draft"
    :busy="restoreBusy"
    @update:model-value="restoring = undefined"
    @confirm="confirmRestore"
  >
    <p>The draft is replaced with the steps of version {{ restoring?.version }}.</p>
    <p>
      Customers are not affected until you publish. Unpublished draft changes are overwritten; Undo
      in the builder does not bring them back.
    </p>
  </ConfirmDialog>
  <PublishReview
    v-model="publishOpen"
    :journey-id="journey.id"
    :name="name"
    :workflow="draft"
    :revision="publishRevision"
    :published-version="publishedVersion"
    :changes="draftDiff"
    :publishing="busy"
    :publish-error="publishError"
    @confirm="confirmPublish"
    @fix="fixPublishIssue"
  />
  <section v-if="issues.length" class="panel">
    <h3>Validation</h3>
    <p v-for="(issue, index) in issues" :key="index" class="error-message">
      {{ issue.node_id || 'Workflow' }}: {{ issue.message }}
    </p>
  </section>
</template>
