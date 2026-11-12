<script setup lang="ts">
import type { DataEntry } from '../lib/data-catalog';
import DataMappingReview from './DataMappingReview.vue';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { api, generationStream, type GenerationProgress, type Journey } from '../lib/api';
import type { CompositionRecord, CompositionState, Proposal } from '../lib/composition';
import { briefExamples } from '../lib/brief-examples';
import GenerationStatus from './GenerationStatus.vue';
import JourneyDiagram from './JourneyDiagram.vue';
import AppIcon from './ui/AppIcon.vue';
const props = defineProps<{ journey?: Journey; readonly?: boolean }>();
const emit = defineEmits<{ saved: [Journey]; open: [Journey]; back: [] }>();
const current = ref(props.journey);
const name = ref(props.journey?.name || '');
const description = ref(props.journey?.description || '');
const state = ref<CompositionState>({ step: 'details' });
const loading = ref(!!props.journey),
  restoreFailed = ref(false),
  working = ref(false),
  saving = ref(false),
  generating = ref(false);
const error = ref(''),
  saveError = ref(''),
  notice = ref(''),
  acknowledged = ref(false);
const dataReady = ref(false);
const catalogEvents = ref<number>();
const catalogFields = ref(0);
async function refreshVocabulary() {
  try {
    const entries = await api<DataEntry[]>('/data-catalog');
    catalogEvents.value = entries.filter((e) => e.enabled && e.kind === 'event').length;
    catalogFields.value = entries.filter((e) => e.enabled && e.kind !== 'event').length;
  } catch (e) {
    error.value = String(e);
    catalogEvents.value = undefined;
  }
}

const opening = ref(0),
  elapsed = ref(0),
  progress = ref<GenerationProgress>({ stage: 'sending' });
const settings = ref<{
  enabled: boolean;
  default_provider: string;
  providers: Record<string, { configured: boolean; model: string }>;
}>();
const configured = computed(
  () =>
    !!settings.value?.enabled && !!settings.value.providers[state.value.provider || '']?.configured,
);
const steps = [
  { id: 'details', label: 'Details' },
  { id: 'method', label: 'Creation method' },
  { id: 'brief', label: 'Brief' },
  { id: 'review', label: 'Review' },
  { id: 'builder', label: 'Builder' },
];
const stepIndex = computed(() =>
  opening.value ? 4 : steps.findIndex((step) => step.id === state.value.step),
);
let saved = '',
  chain: Promise<unknown> = Promise.resolve(),
  timer: ReturnType<typeof setTimeout> | undefined,
  clock: ReturnType<typeof setInterval> | undefined;
let controller: AbortController | undefined,
  mounted = true;
const snapshot = () =>
  JSON.stringify({ state: state.value, name: name.value, description: description.value });
const dirty = computed(() => !!current.value && snapshot() !== saved);
async function persist() {
  clearTimeout(timer);
  const task = chain
    .catch(() => {})
    .then(async () => {
      if (!current.value || restoreFailed.value || props.readonly || snapshot() === saved) return;
      const captured = snapshot(),
        body = JSON.parse(captured);
      saving.value = true;
      try {
        const result = await api<CompositionRecord>(
          `/journeys/${current.value.id}/composition`,
          'PUT',
          { revision: current.value.revision, ...body },
        );
        current.value = result.journey;
        saved = captured;
        saveError.value = '';
        if (mounted) emit('saved', result.journey);
      } catch (cause) {
        saveError.value = String(cause);
        throw cause;
      } finally {
        saving.value = false;
      }
    });
  chain = task;
  return task;
}
function schedule() {
  if (
    current.value &&
    name.value.trim() &&
    !loading.value &&
    !restoreFailed.value &&
    !props.readonly &&
    !opening.value
  ) {
    clearTimeout(timer);
    timer = setTimeout(() => void persist().catch(() => {}), 500);
  }
}
watch(snapshot, schedule);
async function load() {
  loading.value = true;
  restoreFailed.value = false;
  error.value = '';
  if (!props.journey) {
    saved = snapshot();
    loading.value = false;
    return;
  }
  try {
    const result = await api<CompositionRecord>(`/journeys/${props.journey.id}/composition`);
    current.value = result.journey;
    state.value = result.state;
    name.value = result.journey.name;
    description.value = result.journey.description;
    saved = snapshot();
    if (state.value.generating) {
      notice.value =
        'The previous generation was interrupted. Your brief and last saved proposal are preserved. No AI request was restarted.';
      state.value.generating = false;
    }
  } catch (cause) {
    error.value = String(cause);
    restoreFailed.value = true;
  } finally {
    loading.value = false;
  }
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value || generating.value || opening.value) event.preventDefault();
}
onMounted(async () => {
  window.addEventListener('beforeunload', beforeUnload);
  await load();
  try {
    settings.value = await api('/settings/ai');
    await refreshVocabulary();
    if (!state.value.provider)
      state.value.provider = settings.value?.default_provider === 'openai' ? 'openai' : 'claude';
  } catch (cause) {
    error.value = String(cause);
  }
});
onBeforeUnmount(() => {
  mounted = false;
  controller?.abort();
  clearTimeout(timer);
  clearInterval(clock);
  window.removeEventListener('beforeunload', beforeUnload);
  if (dirty.value && !opening.value) void persist().catch(() => {});
});
async function detailsNext() {
  working.value = true;
  error.value = '';
  try {
    if (!current.value) {
      const journey = await api<Journey>('/journeys', 'POST', {
        name: name.value.trim(),
        description: description.value,
        creation_step: 'method',
      });
      emit('open', journey);
    } else {
      state.value.step = 'method';
      await persist();
    }
  } catch (cause) {
    error.value = String(cause);
  } finally {
    working.value = false;
  }
}
async function move(step: CompositionState['step']) {
  if (generating.value || working.value || props.readonly) return;
  state.value.step = step;
  error.value = '';
  await persist().catch(() => {});
}
async function method(method: 'ai' | 'scratch') {
  working.value = true;
  error.value = '';
  try {
    state.value.method = method;
    state.value.step = method === 'ai' ? 'brief' : 'method';
    await persist();
    if (method === 'scratch') await approve();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    working.value = false;
  }
}
async function example(index: number) {
  const sample = briefExamples[index];
  if (!sample || generating.value) return;
  try {
    if (sample.contract) {
      const response = await api<{ brief: string }>('/templates/abandoned-cart-eur');
      state.value.brief = response.brief.replace(/Never publish\.?/gi, '');
      state.value.acceptance_contract = 'cart-recovery-eur.v1';
    } else {
      state.value.brief = sample.brief;
      state.value.acceptance_contract = undefined;
    }
  } catch (cause) {
    error.value = String(cause);
  }
}
async function compose() {
  if (generating.value || !configured.value || props.readonly) return;
  await refreshVocabulary();
  if (!catalogEvents.value) {
    error.value = 'Define or discover workspace events in Settings → Workspace data first.';
    return;
  }
  const modifying = state.value.step === 'review';
  const prompt = (modifying ? state.value.changes : state.value.brief)?.trim();
  if (!prompt) return;
  error.value = '';
  notice.value = '';
  generating.value = true;
  state.value.generating = true;
  progress.value = { stage: 'sending' };
  elapsed.value = 0;
  const started = Date.now();
  clock = setInterval(() => {
    elapsed.value = Math.floor((Date.now() - started) / 1000);
  }, 1000);
  controller = new AbortController();
  try {
    await persist();
    const proposal = await generationStream<Proposal>(
      modifying ? '/ai/journeys/modify' : '/ai/journeys/generate',
      {
        provider: state.value.provider,
        acceptance_contract: state.value.acceptance_contract,
        action: modifying ? 'modify' : 'generate',
        prompt,
        name: name.value,
        description: description.value,
        workflow: modifying
          ? state.value.proposal!.workflow
          : { schema_version: 1, nodes: [], edges: [] },
      },
      (event) => {
        progress.value = event;
      },
      controller.signal,
    );
    progress.value = { stage: 'saving_review' };
    state.value.proposal = proposal;
    state.value.confirmed_references = [];
    dataReady.value = false;
    state.value.step = 'review';
    state.value.generating = false;
    state.value.editing = false;
    state.value.changes = '';
    acknowledged.value = false;
    await persist();
  } catch (cause) {
    error.value = controller.signal.aborted
      ? 'Cancellation requested. The provider may already have processed the request; no automatic retry was made.'
      : String(cause);
    state.value.generating = false;
    if (mounted) await persist().catch(() => {});
  } finally {
    generating.value = false;
    clearInterval(clock);
  }
}
async function changes() {
  state.value.editing = true;
  state.value.changes = '';
  acknowledged.value = false;
  await persist().catch(() => {});
}
async function approve() {
  if (
    props.readonly ||
    generating.value ||
    saveError.value ||
    (state.value.method === 'ai' && !dataReady.value)
  )
    return;
  opening.value = 1;
  error.value = '';
  try {
    await persist();
    opening.value = 2;
    const result = await api<Journey>(
      `/journeys/${current.value!.id}/composition/approve`,
      'POST',
      { revision: current.value!.revision },
    );
    opening.value = 3;
    await nextTick();
    emit('open', result);
  } catch (cause) {
    error.value = String(cause);
    opening.value = 0;
  }
}
async function back() {
  controller?.abort();
  try {
    await persist();
  } catch {
    /* Keep error visible when saving fails. */ return;
  }
  emit('back');
}
</script>
<template>
  <section class="creation-page" aria-label="Create journey workspace">
    <header class="creation-heading">
      <div>
        <button type="button" class="text-button" @click="back">← Journeys</button>
        <h1>{{ name || 'Create a journey' }}</h1>
        <p>Build a customer experience, one step at a time.</p>
      </div>
      <span
        v-if="current"
        class="creation-save"
        :class="{ 'error-message': saveError }"
        role="status"
        >{{
          saveError
            ? 'Not saved'
            : saving
              ? 'Saving…'
              : dirty
                ? 'Unsaved changes'
                : 'All changes saved'
        }}</span
      >
    </header>
    <nav
      class="creation-stepper"
      aria-label="Journey creation steps"
      :style="{ '--active-step': stepIndex }"
    >
      <span class="stepper-indicator" aria-hidden="true" /><span
        v-for="(item, index) in steps"
        :key="item.id"
        :class="{ active: index === stepIndex, complete: index < stepIndex }"
        :aria-current="index === stepIndex ? 'step' : undefined"
        ><b>{{ index < stepIndex ? '✓' : index + 1 }}</b
        >{{ item.label }}</span
      >
    </nav>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="saveError" role="alert" class="error-message">
      {{ saveError }} <button type="button" @click="persist().catch(() => {})">Retry saving</button>
    </p>
    <p v-if="notice" role="status" class="inline-alert">{{ notice }}</p>
    <div v-if="loading" class="creation-surface" role="status">Restoring your creation draft…</div>
    <section v-else-if="restoreFailed" class="creation-surface">
      <h2>Creation draft unavailable</h2>
      <p>Your saved state has not been replaced. Retry when the connection is available.</p>
      <button type="button" @click="load">Retry loading</button>
    </section>
    <section
      v-else-if="opening"
      class="creation-surface opening-journey"
      aria-label="Preparing journey builder"
      aria-busy="true"
    >
      <h2>Preparing your journey</h2>
      <p>Your reviewed plan is being saved before the builder opens.</p>
      <progress :value="opening - 1" max="3" aria-label="Builder preparation progress" />
      <ol>
        <li :class="{ complete: opening > 1 }">Save creation details</li>
        <li :class="{ complete: opening > 2 }">Apply the approved workflow</li>
        <li>Open the builder</li>
      </ol>
    </section>
    <form
      v-else-if="state.step === 'details'"
      class="creation-surface creation-details"
      @submit.prevent="detailsNext"
    >
      <h2>Journey details</h2>
      <p>Give this journey a name your team will recognize.</p>
      <label
        >Journey name<input
          v-model="name"
          required
          maxlength="160"
          :disabled="readonly"
          placeholder="e.g. Trial conversion" /></label
      ><label
        >Description <span class="optional">optional</span
        ><textarea
          v-model="description"
          rows="3"
          maxlength="4000"
          :disabled="readonly"
          placeholder="What should this journey achieve?"
        />
      </label>
      <footer>
        <button type="button" @click="back">Cancel</button
        ><button class="primary" :disabled="working || readonly || !name.trim()">
          {{ working ? 'Saving…' : 'Next' }} <AppIcon name="arrow" :size="16" />
        </button>
      </footer>
    </form>
    <section v-else-if="state.step === 'method'" class="creation-surface creation-methods">
      <h2>How would you like to build it?</h2>
      <p>Start with a blank canvas or review an AI-generated plan.</p>
      <button
        type="button"
        class="creation-method"
        :disabled="working || readonly"
        @click="method('scratch')"
      >
        <AppIcon name="workflow" :size="22" /><span
          ><strong>Start from scratch</strong
          ><small>Add triggers, conditions and messages yourself.</small></span
        ><AppIcon name="arrow" :size="16" /></button
      ><button
        type="button"
        class="creation-method"
        :disabled="working || readonly"
        @click="method('ai')"
      >
        <AppIcon name="sparkles" :size="22" /><span
          ><strong>Create with AI</strong
          ><small>Write a brief, review the diagram and request changes.</small></span
        ><AppIcon name="arrow" :size="16" />
      </button>
      <footer>
        <button type="button" :disabled="working" @click="move('details')">Back</button>
      </footer>
    </section>
    <section v-else-if="generating" class="creation-surface creation-generating">
      <div class="submitted-brief">
        <small>{{ state.step === 'review' ? 'Your requested changes' : 'Your brief' }}</small>
        <p>{{ state.step === 'review' ? state.changes : state.brief }}</p>
      </div>
      <GenerationStatus :progress="progress" :elapsed="elapsed" @cancel="controller?.abort()" />
    </section>
    <section v-else-if="state.step === 'brief'" class="creation-surface brief-page">
      <h2>Describe the journey</h2>
      <p>Explain who enters, what happens next and when the journey should end.</p>
      <section class="brief-examples" aria-label="Example briefs">
        <span>Start with an example</span>
        <div>
          <button
            v-for="(sample, index) in briefExamples"
            :key="sample.title"
            type="button"
            :disabled="readonly"
            @click="example(index)"
          >
            <strong>{{ sample.title }}</strong
            ><small>{{ sample.description }}</small>
          </button>
        </div>
      </section>
      <div class="brief-vocabulary">
        <p v-if="catalogEvents">
          Using {{ catalogEvents }} workspace events and {{ catalogFields }} fields. Write your
          brief in everyday language.
        </p>
        <p v-else class="warning-message">
          Your workspace needs event definitions before AI can connect a brief to real data.
        </p>
        <p class="help-text">
          An administrator can define or discover them in
          <a href="/settings" target="_blank" rel="noopener">Settings → Workspace data</a>.
        </p>
        <button type="button" @click="refreshVocabulary">Refresh workspace data</button>
      </div>
      <form @submit.prevent="compose">
        <label
          >Your brief<textarea
            v-model="state.brief"
            rows="8"
            maxlength="4000"
            required
            :disabled="readonly"
            placeholder="When a customer starts a trial, welcome them by email…"
            @blur="persist().catch(() => {})"
          />
        </label>
        <details class="composer-options">
          <summary>AI options</summary>
          <label
            >Provider<select v-model="state.provider" :disabled="readonly">
              <option value="openai">OpenAI</option>
              <option value="claude">Claude</option>
            </select></label
          ><label
            >Behavior contract<select v-model="state.acceptance_contract" :disabled="readonly">
              <option :value="undefined">General workflow — reviewed scenarios</option>
              <option value="cart-recovery-eur.v1">EUR cart recovery — validated behavior</option>
            </select></label
          >
        </details>
        <p v-if="settings && !configured" class="warning-message">
          This provider needs a server-side key and model before generation is available.
        </p>
        <p class="help-text">
          Your brief, workflow and workspace data definitions are sent. Customer values are
          excluded. You review and approve the result before it opens in the builder.
        </p>
        <footer>
          <button type="button" @click="move('method')">Back</button
          ><button class="primary" :disabled="readonly || !configured || !state.brief?.trim()">
            Generate plan <AppIcon name="arrow" :size="16" />
          </button>
        </footer>
      </form>
    </section>
    <section v-else-if="state.step === 'review' && state.proposal" class="creation-review">
      <div class="creation-surface review-diagram">
        <div class="review-title">
          <div>
            <h2>Review your journey</h2>
            <p>{{ state.proposal.explanation }}</p>
          </div>
          <span class="pill">Not applied</span>
        </div>
        <details class="review-brief">
          <summary>View original brief</summary>
          <p>{{ state.brief }}</p>
        </details>
        <JourneyDiagram :workflow="state.proposal.workflow" />
        <DataMappingReview
          :workflow="state.proposal.workflow"
          :confirmed="state.confirmed_references"
          :readonly="readonly"
          @ready="dataReady = $event"
          @resolved="
            (workflow, confirmed) => {
              state.proposal!.workflow = workflow;
              state.confirmed_references = confirmed;
              acknowledged = false;
              persist().catch(() => {});
            }
          "
        />
      </div>
      <aside class="creation-surface review-decisions">
        <h2>Review & next steps</h2>
        <p>Approval opens an editable draft. Publishing is a separate action in the builder.</p>
        <ul v-if="state.proposal.warnings.length" class="review-warnings">
          <li v-for="warning in state.proposal.warnings" :key="warning">{{ warning }}</li>
        </ul>
        <p v-if="state.proposal.generation?.usage" class="help-text">
          {{ state.proposal.generation.usage.input_tokens.toLocaleString() }} input ·
          {{ state.proposal.generation.usage.output_tokens.toLocaleString() }} output tokens
        </p>
        <template v-if="state.editing"
          ><form @submit.prevent="compose">
            <label
              >What should change?<textarea
                v-model="state.changes"
                rows="4"
                maxlength="4000"
                required
                :disabled="readonly"
                placeholder="Remove the SMS and wait three days instead…"
                @blur="persist().catch(() => {})"
              /></label
            ><button class="primary" :disabled="readonly || !configured || !state.changes?.trim()">
              Update plan</button
            ><button
              type="button"
              @click="
                state.editing = false;
                persist().catch(() => {});
              "
            >
              Cancel changes
            </button>
          </form></template
        >
        <template v-else
          ><label class="acknowledge"
            ><input v-model="acknowledged" type="checkbox" :disabled="readonly" />I reviewed the
            flow, messages and assumptions</label
          ><button
            class="primary"
            :disabled="readonly || !acknowledged || !!saveError || !dataReady"
            @click="approve"
          >
            Approve & open builder</button
          ><button type="button" :disabled="readonly" @click="changes">Request changes</button
          ><button type="button" class="text-button" :disabled="readonly" @click="move('brief')">
            Edit original brief
          </button></template
        >
      </aside>
    </section>
  </section>
</template>
