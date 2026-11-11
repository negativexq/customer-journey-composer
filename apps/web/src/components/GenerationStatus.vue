<script setup lang="ts">
import { computed } from 'vue';
import type { GenerationProgress } from '../lib/api';
const props = defineProps<{ progress: GenerationProgress; elapsed: number; compact?: boolean }>();
defineEmits<{ cancel: [] }>();
const stages: Record<string, string> = {
  sending: 'Sending your brief',
  accepted: 'Request received',
  generating: 'Waiting for the AI provider',
  validating: 'Checking workflow and messages',
  repairing: 'Validation found issues · preparing a correction',
  retry_wait: 'Provider temporarily unavailable · waiting to retry',
};
const elapsedLabel = computed(
  () => `${Math.floor(props.elapsed / 60)}:${String(props.elapsed % 60).padStart(2, '0')}`,
);
</script>
<template>
  <section class="generation-status" :class="{ compact }" aria-label="Journey generation progress">
    <div class="generation-status-heading">
      <strong>Creating your proposal</strong
      ><time aria-label="Elapsed time">{{ elapsedLabel }} elapsed</time>
    </div>
    <div class="generation-activity" aria-hidden="true"><span /></div>
    <p class="generation-current" role="status">
      {{ stages[progress.stage] || 'Waiting for proposal'
      }}<span v-if="progress.attempt"> · attempt {{ progress.attempt }}</span>
    </p>
    <ol v-if="!compact" class="generation-stages" aria-label="Generation steps">
      <li :class="{ complete: progress.stage !== 'sending' }">Request received</li>
      <li
        :class="{
          current: ['generating', 'retry_wait', 'repairing'].includes(progress.stage),
          complete: progress.stage === 'validating',
        }"
      >
        Generate a proposal
      </li>
      <li :class="{ current: progress.stage === 'validating' }">Validate workflow and messages</li>
      <li>Ready for your review</li>
    </ol>
    <p class="help-text">Your draft stays unchanged until you review and apply the proposal.</p>
    <p v-if="elapsed >= 30" class="help-text">
      Still waiting for a result. Complex journeys can take longer; no need to submit again.
    </p>
    <button type="button" @click="$emit('cancel')">Cancel request</button>
  </section>
</template>
