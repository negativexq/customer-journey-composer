<script setup lang="ts">
import { ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import { formatDateTime } from '../lib/format';
import { runContext, runTitle, type TestRunSummary } from '../lib/test-runs';

// Saved test runs, newest first; older ones open on request so the column stays short.
defineProps<{ runs: TestRunSummary[] }>();
const emit = defineEmits<{ open: [string] }>();
const RUN_PAGE = 8;
const runLimit = ref(RUN_PAGE);
</script>
<template>
  <section class="surface recent-test-runs">
    <div class="surface-heading">
      <h2>Recent test runs</h2>
      <span class="count-label">{{ runs.length }}</span>
    </div>
    <div v-if="!runs.length" class="small-empty">
      Saved runs and their results will appear here.
    </div>
    <button
      v-for="r in runs.slice(0, runLimit)"
      :key="r.id"
      class="test-run-row"
      @click="emit('open', r.id)"
    >
      <span class="test-run-symbol"><AppIcon name="flask" :size="15" /></span
      ><span
        ><strong>{{ runTitle(r) }}</strong
        ><small>{{ runContext(r) }} · {{ formatDateTime(r.created_at || '') }}</small></span
      ><StatusBadge :status="r.result" /><AppIcon name="arrow" :size="14" />
    </button>
    <button
      v-if="runs.length > runLimit"
      type="button"
      class="show-more-runs"
      @click="runLimit += RUN_PAGE"
    >
      Show {{ Math.min(RUN_PAGE, runs.length - runLimit) }} more
      <span>{{ runLimit }} of {{ runs.length }}</span>
    </button>
  </section>
</template>
