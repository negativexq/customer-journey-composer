<script setup lang="ts">
// Placeholder for content that is on its way. The label is what assistive technology announces;
// the bars are decoration. It fades in after a beat so fast responses never flash a skeleton.
withDefaults(defineProps<{ label: string; rows?: number; variant?: 'rows' | 'text' | 'cards' }>(), {
  rows: 3,
  variant: 'rows',
});
</script>
<template>
  <div class="skeleton" :class="variant" role="status" aria-busy="true">
    <span class="sr-only">{{ label }}</span>
    <span v-for="i in rows" :key="i" class="skeleton-bar" aria-hidden="true" />
  </div>
</template>
<style scoped>
.skeleton {
  display: grid;
  gap: 12px;
  padding: 4px 0;
  animation: skeleton-in 0.2s 0.15s both;
}
.skeleton-bar {
  display: block;
  height: 34px;
  border-radius: 6px;
  background: linear-gradient(90deg, var(--gray-96), var(--gray-99), var(--gray-96));
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.6s linear infinite;
}
.text {
  gap: 10px;
}
.text .skeleton-bar {
  height: 12px;
  border-radius: 4px;
}
.text .skeleton-bar:nth-child(3n + 2) {
  width: 72%;
}
.text .skeleton-bar:nth-child(3n + 3) {
  width: 88%;
}
.text .skeleton-bar:nth-child(3n + 4) {
  width: 54%;
}
.cards {
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
}
.cards .skeleton-bar {
  height: 86px;
  border-radius: 10px;
}
@keyframes skeleton-in {
  from {
    opacity: 0;
  }
}
@keyframes skeleton-shimmer {
  to {
    background-position: -200% 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .skeleton,
  .skeleton-bar {
    animation: none;
  }
}
</style>
