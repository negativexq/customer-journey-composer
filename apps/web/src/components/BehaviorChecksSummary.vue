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
    <strong>{{
      checks.version === 2 ? 'Reviewed business contract' : 'Independent expected results'
    }}</strong>
    <span v-if="checks.version === 2"
      >{{ checks.case_name }} · {{ (checks.events || []).length }} subsequent events</span
    >
    <span
      >{{ checks.email_count }} email · {{ checks.sms_count }} SMS ·<template
        v-if="checks.push_count !== undefined"
      >
        {{ checks.push_count }} push ·</template
      >
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
    <ul v-if="checks.alternatives?.length" class="contract-clause-list">
      <li v-for="(clause, index) in checks.alternatives" :key="index">
        <strong>{{ clause.name }}</strong
        ><span
          >{{ clause.email_count }} email · {{ clause.sms_count }} SMS ·<template
            v-if="clause.push_count !== undefined"
          >
            {{ clause.push_count }} push ·</template
          >
          {{ clause.outcome }}</span
        ><span
          >{{ (clause.events || []).length }} subsequent events<span
            v-if="clause.duration_seconds !== undefined"
          >
            · {{ clause.duration_seconds / 3600 }} hours</span
          ></span
        >
      </li>
    </ul>
    <span v-if="checks.version === 2 && checks.thresholds?.length"
      >Ineligible outcome: {{ checks.ineligible_outcome }}; no messages or attribute updates.</span
    >
    <small
      >Expectations apply only within each declared situation. Unspecified states and event
      sequences still need review and Test Lab coverage.</small
    >
  </section>
</template>
