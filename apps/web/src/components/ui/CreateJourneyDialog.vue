<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue';
import { api, type Journey } from '../../lib/api';
import AppIcon from './AppIcon.vue';
const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [boolean]; created: [Journey]; templates: [] }>();
const dialog = ref<HTMLDialogElement>();
const input = ref<HTMLInputElement>();
const name = ref('');
const description = ref('');
const busy = ref(false);
const error = ref('');
async function sync() {
  await nextTick();
  if (props.modelValue && !dialog.value?.open) {
    dialog.value?.showModal();
    input.value?.focus();
  } else if (!props.modelValue && dialog.value?.open) dialog.value.close();
}
watch(() => props.modelValue, sync);
onMounted(sync);
async function create() {
  busy.value = true;
  error.value = '';
  try {
    const journey = await api<Journey>('/journeys', 'POST', {
      name: name.value,
      description: description.value,
      tags: [],
    });
    emit('update:modelValue', false);
    emit('created', journey);
    name.value = '';
    description.value = '';
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <dialog ref="dialog" class="app-dialog" @close="emit('update:modelValue', false)">
    <form @submit.prevent="create">
      <header class="dialog-heading">
        <span class="dialog-symbol"><AppIcon name="workflow" :size="24" /></span
        ><button
          class="icon-button"
          type="button"
          aria-label="Close dialog"
          @click="dialog?.close()"
        >
          <AppIcon name="close" />
        </button>
      </header>
      <h2>Create a journey</h2>
      <p>Start with a blank canvas. Add steps yourself or compose with AI.</p>
      <label
        >Journey name<input
          ref="input"
          v-model="name"
          maxlength="160"
          placeholder="e.g. Abandoned cart recovery"
          required /></label
      ><label
        >Description <span class="optional">optional</span
        ><textarea
          v-model="description"
          rows="3"
          maxlength="4000"
          placeholder="What should this journey achieve?"
        />
      </label>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <footer class="dialog-footer">
        <button type="button" @click="dialog?.close()">Cancel</button
        ><button class="primary" :disabled="busy || !name.trim()">
          {{ busy ? 'Creating…' : 'Create journey' }}<AppIcon name="arrow" :size="16" />
        </button>
      </footer>
      <div class="dialog-alternative">
        <span>Looking for a starting point?</span
        ><button
          type="button"
          class="text-button"
          @click="
            dialog?.close();
            emit('templates');
          "
        >
          Browse templates <AppIcon name="arrow" :size="14" />
        </button>
      </div>
    </form>
  </dialog>
</template>
