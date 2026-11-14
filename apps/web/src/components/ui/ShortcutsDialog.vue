<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue';
import AppIcon from './AppIcon.vue';
// Reference card for the builder's keyboard shortcuts, in the notation of the current platform.
const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [boolean] }>();
const dialog = ref<HTMLDialogElement>();
const mod = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl';
const groups = [
  {
    title: 'Editing',
    items: [
      { keys: [mod, 'Z'], label: 'Undo' },
      { keys: [mod, 'Shift', 'Z'], label: 'Redo' },
      { keys: [mod, 'C'], label: 'Copy selected steps' },
      { keys: [mod, 'V'], label: 'Paste' },
      { keys: [mod, 'D'], label: 'Duplicate selected steps' },
      { keys: ['Delete'], label: 'Remove selected steps or connections' },
    ],
  },
  {
    title: 'Selection and navigation',
    items: [
      { keys: [mod, 'A'], label: 'Select every step' },
      { keys: ['Shift', 'Click'], label: 'Add a step to the selection' },
      { keys: [mod, 'K'], label: 'Search journeys' },
      { keys: ['?'], label: 'Show this list' },
    ],
  },
];
async function sync() {
  await nextTick();
  if (props.modelValue && !dialog.value?.open) dialog.value?.showModal();
  else if (!props.modelValue && dialog.value?.open) dialog.value.close();
}
watch(() => props.modelValue, sync);
onMounted(sync);
const close = () => emit('update:modelValue', false);
</script>
<template>
  <dialog
    ref="dialog"
    class="shortcuts-dialog"
    aria-labelledby="shortcuts-title"
    @close="close"
    @click="$event.target === dialog && close()"
  >
    <header>
      <h2 id="shortcuts-title">Keyboard shortcuts</h2>
      <button class="icon-button" type="button" aria-label="Close shortcuts" @click="close">
        <AppIcon name="close" :size="16" />
      </button>
    </header>
    <section v-for="group in groups" :key="group.title">
      <h3>{{ group.title }}</h3>
      <dl>
        <div v-for="item in group.items" :key="item.label">
          <dt>{{ item.label }}</dt>
          <dd>
            <kbd v-for="key in item.keys" :key="key">{{ key }}</kbd>
          </dd>
        </div>
      </dl>
    </section>
  </dialog>
</template>
<style scoped>
.shortcuts-dialog {
  width: min(480px, calc(100vw - 32px));
  padding: 20px 24px 24px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  color: var(--text);
  box-shadow: 0 24px 90px #20263930;
}
.shortcuts-dialog::backdrop {
  background: #1f25394d;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
h2 {
  margin: 0;
  font-size: 16px;
}
h3 {
  margin: 20px 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}
dl {
  margin: 0;
}
dl div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 7px 0;
  border-top: 1px solid var(--border);
}
dt {
  font-size: 13px;
}
dd {
  display: flex;
  gap: 4px;
  margin: 0;
}
kbd {
  min-width: 24px;
  padding: 2px 6px;
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 5px;
  background: var(--page);
  font: inherit;
  font-size: 12px;
  text-align: center;
  color: var(--text);
}
</style>
