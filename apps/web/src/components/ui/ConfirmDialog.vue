<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue';
// Modal confirmation for actions that change what customers experience.
// Cancel is the initial focus and the Escape/backdrop outcome.
const props = defineProps<{
  modelValue: boolean;
  title: string;
  confirmLabel: string;
  danger?: boolean;
  busy?: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [boolean]; confirm: [] }>();
// Several confirmations can live on one page; each needs its own label ids.
const uid = 'confirm-' + Math.random().toString(36).slice(2, 9);
const dialog = ref<HTMLDialogElement>();
const cancelButton = ref<HTMLButtonElement>();
async function sync() {
  await nextTick();
  if (props.modelValue && !dialog.value?.open) {
    dialog.value?.showModal();
    cancelButton.value?.focus();
  } else if (!props.modelValue && dialog.value?.open) dialog.value.close();
}
watch(() => props.modelValue, sync);
onMounted(sync);
const close = () => emit('update:modelValue', false);
function backdrop(event: MouseEvent) {
  if (event.target === dialog.value && !props.busy) close();
}
</script>
<template>
  <dialog
    ref="dialog"
    class="confirm-dialog"
    :aria-labelledby="uid + '-title'"
    :aria-describedby="uid + '-body'"
    @close="close"
    @cancel="busy && $event.preventDefault()"
    @click="backdrop"
  >
    <h2 :id="uid + '-title'">{{ title }}</h2>
    <div :id="uid + '-body'" class="confirm-dialog-body"><slot /></div>
    <footer>
      <button ref="cancelButton" type="button" :disabled="busy" @click="close">Cancel</button
      ><button
        type="button"
        class="primary"
        :class="{ 'confirm-danger': danger }"
        :disabled="busy"
        @click="emit('confirm')"
      >
        {{ busy ? 'Working…' : confirmLabel }}
      </button>
    </footer>
  </dialog>
</template>
<style scoped>
.confirm-dialog {
  width: min(460px, calc(100vw - 32px));
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 24px;
  color: var(--text);
  box-shadow: 0 24px 90px #20263930;
}
.confirm-dialog::backdrop {
  background: color-mix(in srgb, var(--gray-27) 30%, transparent);
}
h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
.confirm-dialog-body {
  margin-top: 10px;
  font-size: 14px;
  line-height: 1.55;
  color: var(--muted);
}
.confirm-dialog-body :deep(p) {
  margin: 0 0 8px;
}
.confirm-dialog-body :deep(strong) {
  color: var(--text);
  font-weight: 600;
}
footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 22px;
}
footer button {
  min-height: 38px;
}
.confirm-danger {
  background: var(--red-56);
  border-color: var(--red-56);
}
.confirm-danger:hover {
  background: var(--red-49);
  border-color: var(--red-49);
}
@media (max-width: 640px) {
  footer {
    flex-direction: column-reverse;
  }
  footer button {
    min-height: 44px;
    width: 100%;
  }
}
</style>
