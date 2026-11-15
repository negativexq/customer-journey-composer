<script setup lang="ts">
import { computed } from 'vue';
import StatusBadge from './StatusBadge.vue';
import type { DeliveryEngagement } from '../../lib/engagement';
const props = defineProps<{
  attempts: unknown[];
  provider?: string;
  response?: string;
  // Reports kept after the provider accepted the message; absent where none can exist.
  engagement?: DeliveryEngagement[];
}>();
const engagementLabel: Record<string, string> = {
  delivered: 'Delivered',
  bounced: 'Bounced',
  opened: 'Opened',
  clicked: 'Clicked a link',
  unsubscribed: 'Unsubscribed',
};
const sourceLabel: Record<string, string> = {
  simulated: 'Simulated by the mock provider',
  provider: 'Reported by the provider',
  tracking: 'Tracked link',
};
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
    <template v-if="engagement">
      <h4>After sending</h4>
      <p v-if="!engagement.length" class="muted">
        Nothing reported yet. Opens, clicks and the provider's delivery report appear here.
      </p>
      <ol v-else class="engagement-list">
        <li v-for="item in engagement" :key="item.kind">
          <div>
            <strong>{{ engagementLabel[item.kind] || item.kind }}</strong
            ><small>{{ sourceLabel[item.source] || item.source }}</small>
          </div>
          <small>{{ time(item.occurred_at) }}</small>
          <p v-if="item.detail?.url">{{ item.detail.url }}</p>
          <p v-if="item.detail?.reason">{{ item.detail.reason }}</p>
        </li>
      </ol>
      <p v-if="engagement.some((e) => e.kind === 'opened')" class="muted">
        Each kind is kept once, from its first report. Opens count when the mail client loads
        images, which some clients do on their own.
      </p>
    </template>
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
  background: var(--gray-99);
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
h4 {
  margin: 20px 0 10px;
  font-size: 13px;
}
.engagement-list li > div small {
  margin: 0;
}
.attempt-provider {
  color: var(--muted);
  margin-bottom: 12px;
}
</style>
