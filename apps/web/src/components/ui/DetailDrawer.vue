<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue';
import AppIcon from './AppIcon.vue';
const props = defineProps<{ modelValue: boolean; title: string }>();
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement>();
async function sync() {
  await nextTick();
  if (props.modelValue && !dialog.value?.open) dialog.value?.showModal();
  else if (!props.modelValue && dialog.value?.open) dialog.value.close();
}
watch(() => props.modelValue, sync);
onMounted(sync);
</script>
<template>
  <dialog
    ref="dialog"
    class="detail-drawer"
    aria-labelledby="detail-drawer-title"
    @close="emit('close')"
  >
    <header>
      <div>
        <h2 id="detail-drawer-title">{{ title }}</h2>
      </div>
      <button class="icon-button" aria-label="Close activity inspector" @click="emit('close')">
        <AppIcon name="close" :size="18" />
      </button>
    </header>
    <div class="detail-drawer-body"><slot /></div>
  </dialog>
</template>
<style scoped>
.detail-drawer {
  position: fixed;
  inset: 0 0 0 auto;
  width: min(640px, 100vw);
  height: 100dvh;
  max-height: 100dvh;
  max-width: 100vw;
  margin: 0;
  border: 0;
  border-left: 1px solid var(--border);
  padding: 0;
  background: var(--surface);
  color: var(--text);
  box-shadow: -12px 0 60px #17213b18;
}
.detail-drawer::backdrop {
  background: color-mix(in srgb, var(--gray-25) 27%, transparent);
}
header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: var(--surface);
  z-index: 2;
}
h2 {
  font-size: 20px;
  margin: 8px 0 0;
}
.detail-drawer-body {
  padding: 24px;
}
</style>
