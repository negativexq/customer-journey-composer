<script setup lang="ts">
import MetricCard from './ui/MetricCard.vue';
import { metric, type Insights } from '../lib/insights';

// The four headline counts of the selected cohort. Each opens the executions behind it.
const props = defineProps<{ insights: Insights }>();
const emit = defineEmits<{ browse: [state: string, reason: string] }>();
const value = (name: string) => metric(props.insights, 'execution', name);
</script>
<template>
  <div class="observe-kpis">
    <MetricCard
      label="Journey entries"
      :value="value('entries')"
      :caption="
        insights.unique_customers === null
          ? 'Retries count once'
          : insights.unique_customers + ' unique customers'
      "
      icon="users"
      @select="emit('browse', '', '')"
    /><MetricCard
      label="Goal reached"
      :value="value('converted')"
      :caption="
        (value('entries') ? ((100 * value('converted')) / value('entries')).toFixed(1) : '0') +
        '% of selected entries'
      "
      icon="chart"
      tone="green"
      @select="emit('browse', '', 'converted')"
    /><MetricCard
      label="In progress"
      :value="value('RUNNING') + value('WAITING')"
      :caption="value('WAITING') + ' waiting now in this cohort'"
      icon="clock"
      @select="emit('browse', 'active', '')"
    /><MetricCard
      label="Needs attention"
      :value="value('FAILED')"
      :caption="
        insights.journey_status === 'Paused'
          ? 'Journey paused · timers held'
          : insights.overdue + ' waits past due by over 30s'
      "
      icon="events"
      tone="amber"
      @select="emit('browse', 'FAILED', '')"
    />
  </div>
</template>
