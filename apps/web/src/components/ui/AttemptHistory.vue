<script setup lang="ts">
import { computed } from 'vue';
import StatusBadge from './StatusBadge.vue';
const props = defineProps<{ attempts: unknown[]; provider?: string; response?: string }>();
const rows = computed(() =>
  props.attempts.filter((a): a is Record<string, unknown> => !!a && typeof a === 'object'),
);
const time = (value: unknown) =>
  typeof value === 'string'
    ? new Date(value).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC'
    : 'Time not recorded';
</script>
<template>
  <section class="attempt-history">
    <p v-if="provider" class="attempt-provider">Provider: {{ provider }}</p>
    <p v-if="!rows.length" class="muted">No provider attempts recorded.</p>
    <ol>
      <li v-for="(attempt, index) in rows" :key="index">
        <div>
          <strong>Attempt {{ attempt.attempt || index + 1 }}</strong
          ><StatusBadge :status="typeof attempt.status === 'string' ? attempt.status : 'UNKNOWN'" />
        </div>
        <small
          >{{ time(attempt.started_at || attempt.at)
          }}<span v-if="attempt.provider_code"> · Code {{ attempt.provider_code }}</span></small
        >
        <p v-if="attempt.response">{{ attempt.response }}</p>
      </li>
    </ol>
    <p v-if="response" class="attempt-response">{{ response }}</p>
  </section>
</template>
<style scoped>
.attempt-history {
  margin-top: 16px;
}
ol {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 10px;
}
li {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 13px;
  background: #fafbfe;
}
li > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}
small {
  display: block;
  margin-top: 8px;
  color: var(--muted);
  font-size: 12px;
}
p {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}
.attempt-provider {
  color: var(--muted);
  margin-bottom: 12px;
}
</style>
