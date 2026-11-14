<script setup lang="ts">
import AppIcon from './AppIcon.vue';
// What a screen says when it has nothing to show: what belongs here, and the next step if there
// is one. `level` is the heading level that fits the surrounding outline.
withDefaults(defineProps<{ icon: string; title: string; level?: 2 | 3; compact?: boolean }>(), {
  level: 3,
});
</script>
<template>
  <div class="empty" :class="{ compact }">
    <span class="empty-mark" aria-hidden="true"
      ><AppIcon :name="icon" :size="compact ? 18 : 22"
    /></span>
    <component :is="'h' + level" class="empty-title">{{ title }}</component>
    <p v-if="$slots.default" class="empty-copy"><slot /></p>
    <div v-if="$slots.action" class="empty-action"><slot name="action" /></div>
  </div>
</template>
<style scoped>
.empty {
  display: grid;
  justify-items: center;
  align-content: center;
  gap: 6px;
  padding: 48px 24px;
  text-align: center;
}
.empty.compact {
  padding: 24px 16px;
}
.empty-mark {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 8px;
  border-radius: 12px;
  background: var(--accent-soft);
  color: var(--accent-text);
}
.compact .empty-mark {
  width: 36px;
  height: 36px;
  margin-bottom: 4px;
  border-radius: 9px;
}
.empty-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}
.compact .empty-title {
  font-size: 13px;
}
.empty-copy {
  margin: 0;
  max-width: 46ch;
  font-size: 13px;
  line-height: 1.55;
  color: var(--muted);
}
.empty-action {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  margin-top: 12px;
}
</style>
