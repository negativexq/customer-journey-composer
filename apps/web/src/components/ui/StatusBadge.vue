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
// Status codes stay codes in the API; people read them as words.
const named: Record<string, string> = {
  TEST_ACCEPTED: 'Accepted',
  TEST_PENDING: 'Pending',
  PASS: 'Passed',
  FAIL: 'Failed',
  CONDITION_NOT_MET: 'Not eligible',
  MANUALLY_CANCELLED: 'Cancelled',
};
const text = computed(() => {
  if (props.label) return props.label;
  const code = props.status.toUpperCase();
  if (named[code]) return named[code];
  const words = props.status.replace(/_/g, ' ').toLowerCase();
  return words.charAt(0).toUpperCase() + words.slice(1);
});
</script>
<template>
  <span class="status-badge" :class="'tone-' + tone" :data-status="status"><i />{{ text }}</span>
</template>
