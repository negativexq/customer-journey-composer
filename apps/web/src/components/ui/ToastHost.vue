<script setup lang="ts">
import AppIcon from './AppIcon.vue';
import { announcement, dismissToast, holdToast, releaseToast, toasts } from '../../lib/toast';
</script>
<template>
  <div class="toast-host">
    <p class="sr-only" aria-live="polite">{{ announcement }}</p>
    <TransitionGroup name="toast">
      <div
        v-for="item in toasts"
        :key="item.id"
        class="toast"
        data-testid="toast"
        @mouseenter="holdToast(item.id)"
        @mouseleave="releaseToast(item.id)"
        @focusin="holdToast(item.id)"
        @focusout="releaseToast(item.id)"
      >
        <span class="toast-mark"><AppIcon name="check" :size="13" /></span>
        <span class="toast-message">{{ item.message }}</span>
        <button
          class="icon-button"
          type="button"
          aria-label="Dismiss notification"
          @click="dismissToast(item.id)"
        >
          <AppIcon name="close" :size="14" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
<style scoped>
.toast-host {
  position: fixed;
  z-index: 95;
  left: 50%;
  bottom: 24px;
  transform: translateX(-50%);
  display: grid;
  justify-items: center;
  gap: 8px;
  width: min(440px, calc(100vw - 24px));
  pointer-events: none;
}
.toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  padding: 8px 8px 8px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  font-weight: 500;
  box-shadow:
    0 12px 32px #17213b24,
    0 2px 6px #17213b14;
}
.toast-mark {
  display: grid;
  place-items: center;
  flex: 0 0 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--green-54);
  color: var(--on-solid);
}
.toast-message {
  min-width: 0;
  overflow-wrap: anywhere;
}
.toast .icon-button {
  flex: 0 0 auto;
  width: 26px;
  height: 26px;
  min-height: 26px;
}
.toast-enter-active,
.toast-leave-active,
.toast-move {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
.toast-leave-active {
  position: absolute;
}
@media (prefers-reduced-motion: reduce) {
  .toast-enter-active,
  .toast-leave-active,
  .toast-move {
    transition: none;
  }
}
</style>
