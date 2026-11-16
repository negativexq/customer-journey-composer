<script setup lang="ts">
import { computed } from 'vue';
import { metric, reasonLabel, type Insights } from '../lib/insights';

// Why runs ended, each reason opening its executions, and how long finished runs took.
const props = defineProps<{ insights: Insights }>();
const emit = defineEmits<{ browse: [string] }>();
const value = (name: string) => metric(props.insights, 'execution', name);
const exitReasons = computed(() =>
  props.insights.metrics.filter((m) => m.dimension === 'exit_reason' && m.value > 0),
);
</script>
<template>
  <section class="surface">
    <div class="surface-heading"><h3>Journey outcomes</h3></div>
    <button v-for="r in exitReasons" :key="r.key" class="list-row" @click="emit('browse', r.key)">
      <span>{{ reasonLabel(r.key) }}</span
      ><strong>{{ r.value }}</strong>
    </button>
    <p class="table-caption">
      {{ value('finished_count') }} finished runs · mean terminal duration
      {{ (value('duration_ms') / Math.max(1, value('finished_count')) / 1000).toFixed(1) }}
      seconds
    </p>
  </section>
</template>
