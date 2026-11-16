<script setup lang="ts">
import { ref, watch } from 'vue';
import AppSelect from './ui/AppSelect.vue';
import AppIcon from './ui/AppIcon.vue';
import { kinds, labels, ports, type Kind } from '../lib/workflow';
import { branchLabel, descriptions, iconFor, stepGroup } from '../lib/step-catalog';

// Puts a new step on an existing connection and asks which of its outputs continues on.
const emit = defineEmits<{ insert: [Kind, string] }>();
const dialog = ref<HTMLDialogElement>();
const kind = ref<Kind>('wait');
const port = ref('');
const insertKinds = kinds.filter(
  (candidate) => candidate !== 'event_trigger' && candidate !== 'exit',
);
watch(kind, (next) => {
  port.value = ports(next).length > 1 ? '' : ports(next)[0]!;
});
function open() {
  kind.value = 'wait';
  port.value = ports(kind.value)[0]!;
  dialog.value?.showModal();
}
function submit() {
  if (ports(kind.value).includes(port.value)) emit('insert', kind.value, port.value);
}
defineExpose({ open, close: () => dialog.value?.close() });
</script>
<template>
  <dialog ref="dialog" class="insert-step-dialog" aria-labelledby="insert-step-title">
    <form @submit.prevent="submit">
      <div class="surface-heading">
        <h2 id="insert-step-title">Insert a step</h2>
        <button
          type="button"
          class="icon-button"
          aria-label="Close insertion"
          @click="dialog?.close()"
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
          v-model="kind"
          :options="
            insertKinds.map((candidate) => ({
              value: candidate,
              label: labels[candidate],
              description: descriptions[candidate],
              icon: iconFor[candidate],
              group: stepGroup[candidate],
            }))
          "
      /></label>
      <label
        >Continue through<AppSelect
          v-model="port"
          placeholder="Select an output"
          :options="
            ports(kind).map((output) => ({ value: output, label: branchLabel(output) || output }))
          "
      /></label>
      <p v-if="ports(kind).length > 1" class="inline-alert">
        Connect the other output before publishing. It is left open for your decision.
      </p>
      <div class="toolbar">
        <button type="button" @click="dialog?.close()">Cancel</button
        ><button class="primary" :disabled="!port">Insert step</button>
      </div>
    </form>
  </dialog>
</template>
