<script setup lang="ts">
import { ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import { kinds, labels, type Kind } from '../lib/workflow';
import { descriptions, iconFor } from '../lib/step-catalog';

// Picks the type of the step that continues from an open output.
defineProps<{ title: string }>();
const emit = defineEmits<{ choose: [Kind] }>();
const dialog = ref<HTMLDialogElement>();
// A journey has one entry, so another trigger is never offered.
const addKinds = kinds.filter((kind) => kind !== 'event_trigger');
defineExpose({
  open: () => dialog.value?.showModal(),
  close: () => dialog.value?.close(),
});
</script>
<template>
  <dialog ref="dialog" class="add-step-dialog" aria-labelledby="add-step-title">
    <div class="surface-heading">
      <div>
        <h2 id="add-step-title">Add a step</h2>
        <p>{{ title }}</p>
      </div>
      <button
        type="button"
        class="icon-button"
        aria-label="Close step picker"
        @click="dialog?.close()"
      >
        <AppIcon name="close" />
      </button>
    </div>
    <div class="add-step-options">
      <button v-for="kind in addKinds" :key="kind" type="button" @click="emit('choose', kind)">
        <span class="step-type-icon" :class="kind"
          ><AppIcon :name="iconFor[kind]" :size="17"
        /></span>
        <span
          ><strong>{{ labels[kind] }}</strong
          ><small>{{ descriptions[kind] }}</small></span
        >
      </button>
    </div>
  </dialog>
</template>
