<script setup lang="ts">
import { computed } from 'vue';
import AppIcon from './AppIcon.vue';
const props = defineProps<{ decision: unknown }>();
const data = computed(() =>
  props.decision && typeof props.decision === 'object'
    ? (props.decision as Record<string, unknown>)
    : {},
);
const reasons: Record<string, string> = {
  eligible: 'This message meets the sending rules.',
  consent_denied: 'The customer has not permitted this channel.',
  consent_revoked: 'Channel permission was withdrawn before dispatch.',
  frequency_cap: 'The customer has reached the sending limit.',
  execution_terminated: 'The journey ended before this message could be sent.',
  missing_personalization: 'Required personalization values are missing.',
  invalid_recipient: 'A valid address or phone number is required.',
  recipient_changed: 'The recipient changed after the message was prepared.',
};
const flag = (value: unknown, yes: string, no: string) =>
  value === true ? yes : value === false ? no : 'Not evaluated';
</script>
<template>
  <div class="decision-summary">
    <div class="decision-heading">
      <AppIcon :name="data.eligible === false ? 'shield' : 'check'" :size="18" /><strong>{{
        flag(data.eligible, 'Eligible to send', 'Sending blocked')
      }}</strong>
    </div>
    <p>
      {{
        typeof data.reason === 'string'
          ? reasons[data.reason] || data.reason.replaceAll('_', ' ')
          : 'No sending decision recorded.'
      }}
    </p>
    <dl>
      <div>
        <dt>Channel consent</dt>
        <dd>{{ flag(data.consent, 'Granted', 'Denied') }}</dd>
      </div>
      <div>
        <dt>Frequency cap</dt>
        <dd>{{ flag(data.frequency_cap, 'Within limit', 'Limit reached') }}</dd>
      </div>
    </dl>
    <details>
      <summary>Decision payload</summary>
      <pre>{{ JSON.stringify(decision, null, 2) }}</pre>
    </details>
  </div>
</template>
<style scoped>
.decision-summary {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  margin-top: 14px;
  background: var(--gray-99);
}
.decision-heading {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 13px;
  color: var(--text);
}
p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
dl {
  margin: 16px 0;
  display: grid;
  gap: 12px;
  font-size: 12px;
}
dl div {
  display: flex;
  gap: 16px;
  justify-content: space-between;
}
dt {
  color: var(--muted);
}
dd {
  margin: 0;
  font-weight: 500;
}
summary {
  font-size: 12px;
  cursor: pointer;
  color: var(--muted);
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
