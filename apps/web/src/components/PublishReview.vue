<script setup lang="ts">
import { errorMessage } from '../lib/format';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { api } from '../lib/api';
import type { Workflow } from '../lib/workflow';
import AppIcon from './ui/AppIcon.vue';
import WorkflowChanges from './WorkflowChanges.vue';
import { plural } from '../lib/format';
import type { WorkflowDiff } from '../lib/version-diff';
const props = defineProps<{
  modelValue: boolean;
  journeyId: string;
  name: string;
  workflow: Workflow;
  revision: number;
  publishedVersion?: number;
  // Draft compared with the latest published version; absent before the first publish.
  changes?: WorkflowDiff;
  publishing?: boolean;
  publishError?: string;
}>();
const emit = defineEmits<{ 'update:modelValue': [boolean]; confirm: []; fix: [string] }>();
type Check = { ready: boolean; issues: { code: string; message: string }[]; policy: string };
const dialog = ref<HTMLDialogElement>();
const gates = ref<Check>();
const loading = ref(false);
const error = ref('');
const acknowledged = ref(false);
let request = 0;
let controller: AbortController | undefined;
const graphValid = computed(
  () => !!gates.value && !gates.value.issues.some((i) => i.code === 'workflow'),
);
const checks = computed(() => [
  {
    label: 'Workflow & message configuration',
    needed: true,
    passed: graphValid.value,
    checked: !!gates.value,
  },
  {
    label: 'Required acceptance scenarios for this workflow',
    needed: true,
    passed: graphValid.value && !gates.value?.issues.some((i) => i.code.startsWith('test_')),
    checked: graphValid.value,
  },
  {
    label: 'Email provider configuration',
    needed: props.workflow.nodes.some((node) => node.type === 'email'),
    passed: graphValid.value && !gates.value?.issues.some((i) => i.code === 'email_provider'),
    checked: graphValid.value,
  },
]);
async function check() {
  controller?.abort();
  controller = new AbortController();
  const current = ++request;
  loading.value = true;
  error.value = '';
  gates.value = undefined;
  acknowledged.value = false;
  try {
    const result = await api<Check>(
      `/journeys/${props.journeyId}/publish-check`,
      'GET',
      undefined,
      {},
      30000,
      controller.signal,
    );
    if (current === request) gates.value = result;
  } catch (cause) {
    if (current === request && !controller.signal.aborted) error.value = errorMessage(cause);
  } finally {
    if (current === request) loading.value = false;
  }
}
function close() {
  request++;
  controller?.abort();
  emit('update:modelValue', false);
}
function fix(code: string) {
  close();
  emit('fix', code);
}
async function sync() {
  await nextTick();
  if (props.modelValue && !dialog.value?.open) {
    dialog.value?.showModal();
    await check();
  } else if (!props.modelValue && dialog.value?.open) dialog.value.close();
}
watch(() => props.modelValue, sync);
onMounted(sync);
onBeforeUnmount(() => controller?.abort());
</script>
<template>
  <dialog
    ref="dialog"
    class="publish-review"
    aria-labelledby="publish-review-title"
    @close="close"
    @cancel="publishing && $event.preventDefault()"
  >
    <header>
      <div>
        <span class="eyebrow">PUBLISH / PREFLIGHT</span>
        <h2 id="publish-review-title">Review before publishing</h2>
        <p>{{ name }} · draft revision {{ revision }}</p>
      </div>
      <button
        class="icon-button"
        aria-label="Close publish review"
        :disabled="publishing"
        @click="close"
      >
        <AppIcon name="close" />
      </button>
    </header>
    <div class="publish-review-body">
      <div v-if="loading" class="publish-checking" role="status" aria-busy="true">
        Checking workflow, test evidence and provider configuration…
      </div>
      <p v-if="error || publishError" class="inline-alert error-alert" role="alert">
        {{ error || publishError }}
      </p>
      <section class="publish-changes" aria-label="What this publish changes">
        <h3>What this publish changes</h3>
        <p v-if="!publishedVersion">
          This is the first version. {{ plural(workflow.nodes.length, 'step') }} go live for new
          entries.
        </p>
        <p v-else-if="!changes">
          The comparison with version {{ publishedVersion }} is not available.
        </p>
        <p v-else-if="!changes.total">
          Nothing behaves differently from version {{ publishedVersion }}. Publishing creates a new
          version with the same steps.
        </p>
        <template v-else>
          <p>
            {{ plural(changes.total, 'change') }} since version {{ publishedVersion }}. New entries
            get them; customers already in the journey keep their version.
          </p>
          <WorkflowChanges :diff="changes" />
        </template>
      </section>
      <div class="publish-review-grid">
        <section>
          <h3>Readiness checks</h3>
          <div v-for="item in checks" :key="item.label" class="preflight-row">
            <AppIcon
              :name="!item.checked ? 'clock' : item.passed ? 'check' : 'close'"
              :size="17"
            /><span>{{ item.label }}</span
            ><strong :class="{ passed: item.needed && item.checked && item.passed }">{{
              !item.needed
                ? 'Not required'
                : !item.checked
                  ? 'Not checked'
                  : item.passed
                    ? 'Passed'
                    : 'Blocked'
            }}</strong>
          </div>
          <section v-if="gates?.issues.length" class="publish-blockers">
            <h3>
              {{ gates.issues.length }} item{{ gates.issues.length === 1 ? '' : 's' }} to resolve
            </h3>
            <div v-for="(issue, index) in gates.issues" :key="index">
              <p>{{ issue.message }}</p>
              <button class="text-button" @click="fix(issue.code)">
                {{
                  issue.code.startsWith('test_')
                    ? 'Open Test Lab'
                    : issue.code === 'email_provider'
                      ? 'Open settings'
                      : 'Review workflow'
                }}<AppIcon name="arrow" :size="14" />
              </button>
            </div>
          </section>
          <p class="preflight-note">
            Provider configuration is checked here; no test message is sent.
          </p>
        </section>
        <aside class="publish-impact">
          <h3>Version & sending rules</h3>
          <dl>
            <div>
              <dt>Version</dt>
              <dd>New immutable snapshot</dd>
            </div>
            <div>
              <dt>Running customers</dt>
              <dd>
                {{
                  publishedVersion ? 'Stay on their original version' : 'No published version yet'
                }}
              </dd>
            </div>
            <div>
              <dt>Workflow</dt>
              <dd>{{ workflow.nodes.length }} steps · {{ workflow.edges.length }} connections</dd>
            </div>
            <div>
              <dt>Global exit</dt>
              <dd>{{ workflow.global_exit?.event_name || 'No global exit event configured' }}</dd>
            </div>
            <div>
              <dt>Re-entry</dt>
              <dd>
                {{
                  workflow.reentry?.mode === 'cooldown'
                    ? 'After ' + (workflow.reentry.cooldown_seconds || 0) / 86400 + ' days'
                    : workflow.reentry?.mode?.replaceAll('_', ' ') || 'After completion'
                }}
              </dd>
            </div>
          </dl>
          <div class="runtime-policy">
            <AppIcon name="shield" :size="18" />
            <p>
              Consent and workspace frequency caps are enforced before dispatch. Personalization is
              checked against each customer.
            </p>
          </div>
        </aside>
      </div>
      <label class="publish-acknowledgement"
        ><input v-model="acknowledged" type="checkbox" :disabled="!gates?.ready || publishing" />I
        reviewed this version and understand that publishing enables new enrollments.</label
      >
    </div>
    <footer>
      <button :disabled="publishing" @click="close">Cancel</button
      ><button :disabled="loading || publishing" @click="check">Recheck</button
      ><button
        class="primary"
        :disabled="!gates?.ready || !acknowledged || loading || publishing"
        @click="emit('confirm')"
      >
        <AppIcon name="upload" :size="16" />{{ publishing ? 'Publishing…' : 'Publish version' }}
      </button>
    </footer>
  </dialog>
</template>
<style scoped>
.publish-review {
  width: min(920px, calc(100vw - 32px));
  max-height: calc(100dvh - 32px);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 0;
  color: var(--text);
  box-shadow: 0 24px 90px #20263930;
}
.publish-review::backdrop {
  background: color-mix(in srgb, var(--gray-27) 30%, transparent);
}
header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 24px;
  border-bottom: 1px solid var(--border);
}
header h2 {
  font-size: 21px;
  margin-top: 8px;
  font-weight: 600;
}
header p {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--muted);
}
.publish-review-body {
  overflow: auto;
  max-height: calc(100dvh - 230px);
  padding: 24px;
}
.publish-review-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  gap: 30px;
}
h3 {
  font-size: 14px;
  margin-bottom: 16px;
}
.preflight-row {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 15px 0;
  border-bottom: 1px solid var(--border);
  font-size: 13px;
}
.preflight-row > span {
  flex: 1;
}
.preflight-row strong {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
}
.preflight-row .passed {
  color: var(--green-54-text);
}
.publish-impact {
  background: var(--page);
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 9px;
}
dl {
  display: grid;
  gap: 16px;
  font-size: 12px;
}
dt {
  color: var(--muted);
}
dd {
  margin: 5px 0 0;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.runtime-policy {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  color: var(--muted);
  padding-top: 15px;
  border-top: 1px solid var(--border);
}
.runtime-policy svg {
  flex-shrink: 0;
}
.runtime-policy p {
  font-size: 12px;
  line-height: 1.7;
  margin: 0;
}
.publish-blockers {
  margin-top: 22px;
  border: 1px solid var(--amber-90);
  border-radius: 8px;
  padding: 15px;
  background: var(--gray-99b);
}
.publish-blockers p {
  font-size: 13px;
  line-height: 1.6;
}
.publish-blockers > div + div {
  border-top: 1px solid var(--amber-90);
  margin-top: 12px;
}
.preflight-note {
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  margin-top: 16px;
}
.publish-acknowledgement {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
  font-size: 13px;
  line-height: 1.6;
}
.publish-acknowledgement input {
  width: 17px;
  min-height: 17px;
  flex-shrink: 0;
  margin-top: 2px;
}
.publish-checking {
  font-size: 13px;
  padding-bottom: 18px;
  color: var(--muted);
}
footer {
  display: flex;
  justify-content: flex-end;
  gap: 9px;
  padding: 18px 24px;
  border-top: 1px solid var(--border);
  background: var(--gray-99);
}
footer button {
  min-height: 40px;
}
@media (max-width: 700px) {
  .publish-review-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
  header,
  .publish-review-body {
    padding: 18px;
  }
  header h2 {
    font-size: 18px;
  }
  footer {
    padding: 14px;
  }
}
.publish-changes {
  margin-bottom: 20px;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--page);
}
.publish-changes h3 {
  margin: 0 0 6px;
  font-size: 14px;
}
.publish-changes p {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--muted);
}
</style>
