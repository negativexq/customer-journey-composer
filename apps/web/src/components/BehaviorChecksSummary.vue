<script setup lang="ts">
import type { BehaviorChecks } from '../lib/workflow';
defineProps<{ checks?: BehaviorChecks }>();
</script>
<template>
  <section
    v-if="checks"
    class="behavior-checks-summary"
    aria-label="Reviewed behavior expectations"
  >
    <strong>Independent expected results</strong>
    <span
      >{{ checks.email_count }} email · {{ checks.sms_count }} SMS ·
      {{
        checks.outcome === 'completed'
          ? 'Completed without conversion'
          : checks.outcome === 'converted'
            ? 'Goal reached'
            : checks.outcome === 'condition_not_met'
              ? 'Not eligible'
              : 'Cancelled'
      }}</span
    >
    <span v-if="checks.duration_seconds !== undefined"
      >Expected duration: {{ checks.duration_seconds / 3600 }} hours</span
    >
    <span v-if="checks.thresholds?.length"
      >{{ checks.thresholds?.length }} exact eligibility boundaries checked below, at and above each
      limit.</span
    >
    <small
      >For your reviewed synthetic customer. Other profiles and event sequences still need Test Lab
      coverage.</small
    >
  </section>
</template>
