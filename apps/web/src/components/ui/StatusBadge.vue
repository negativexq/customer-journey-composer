<script setup lang="ts">
import { computed } from 'vue';
const props = defineProps<{ status: string; label?: string }>();
const tone = computed(() => {
  const s = props.status.toUpperCase();
  if (['ACTIVE', 'RUNNING', 'ACCEPTED', 'TEST_ACCEPTED', 'PASS'].includes(s)) return 'green';
  if (['FAILED', 'FAIL', 'REJECTED'].includes(s)) return 'red';
  if (['PAUSED', 'WAITING', 'BLOCKED', 'UNKNOWN', 'TEST_PENDING'].includes(s)) return 'amber';
  if (['COMPLETED', 'SIMULATED'].includes(s)) return 'blue';
  if (['SUNSET', 'EXITED'].includes(s)) return 'purple';
  return 'neutral';
});
</script>
<template>
  <span class="status-badge" :class="'tone-' + tone"><i />{{ label || status }}</span>
</template>
