<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import { errorMessage, formatDateTime } from '../lib/format';
import { toast } from '../lib/toast';
import AppSelect from './ui/AppSelect.vue';

type Channel = {
  provider: 'smtp' | 'mock_sms';
  channel: 'email' | 'sms';
  state:
    'healthy' | 'throttled' | 'rate_limited' | 'credentials_refused' | 'circuit_open' | 'probing';
  until?: string;
  rate_per_second: number;
  simulate?: string;
  simulate_until?: string;
  calls_last_10m: number;
  accepted_last_10m: number;
  failed_last_10m: number;
  held_last_10m: number;
  last_problem?: { code: number; response: string; at: string };
};
const channels = ref<Channel[]>([]);
const error = ref('');
const saving = ref('');
const drafts = ref<Record<string, { rate: number; simulate: string; minutes: number }>>({});
let timer: ReturnType<typeof setInterval> | undefined;

const states: Record<Channel['state'], { label: string; tone: string; help: string }> = {
  healthy: { label: 'Sending', tone: 'good', help: 'Calls go out at up to the rate below.' },
  rate_limited: {
    label: 'At its rate',
    tone: 'good',
    help: 'The last second used the whole rate; the next calls wait a moment.',
  },
  throttled: {
    label: 'Slowed by the provider',
    tone: 'warn',
    help: 'The provider answered 429. Every worker waits for its Retry-After; no message uses an attempt.',
  },
  credentials_refused: {
    label: 'Credentials refused',
    tone: 'bad',
    help: 'The provider answered 401. Nothing is sent until a test call, one every 30 seconds, is accepted.',
  },
  circuit_open: {
    label: 'Provider failing',
    tone: 'bad',
    help: 'Most recent calls failed. Messages wait in the queue without using attempts until test calls go through.',
  },
  probing: {
    label: 'Testing the provider',
    tone: 'warn',
    help: 'Three test calls decide whether the channel sends again.',
  },
};
const faults = [
  { value: '', label: 'No problem', description: 'The provider answers normally' },
  { value: 'throttle', label: 'Rate limit (429)', description: 'Asks to slow down for 5 seconds' },
  { value: 'outage', label: 'Outage (503)', description: 'Every call fails' },
  { value: 'credentials', label: 'Refused key (401)', description: 'The credentials are refused' },
];
// The provider's answer usually names its code already.
const problemText = (p: NonNullable<Channel['last_problem']>) =>
  p.code && !p.response.includes(String(p.code)) ? `${p.response} (${p.code})` : p.response;
const label = (c: Channel) => (c.channel === 'email' ? 'Email' : 'SMS');
const anyTrouble = computed(() => channels.value.some((c) => states[c.state].tone !== 'good'));

async function load() {
  try {
    channels.value = await api<Channel[]>('/delivery/channels');
    // A form being edited keeps its values while the status refreshes.
    for (const c of channels.value)
      drafts.value[c.provider] ??= {
        rate: c.rate_per_second,
        simulate: c.simulate ?? '',
        minutes: 5,
      };
    error.value = '';
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function save(c: Channel) {
  const draft = drafts.value[c.provider];
  saving.value = c.provider;
  try {
    channels.value = await api<Channel[]>(`/delivery/channels/${c.provider}`, 'PUT', {
      rate_per_second: draft.rate,
      simulate: draft.simulate,
      simulate_minutes: draft.simulate ? draft.minutes : 0,
    });
    toast(
      draft.simulate
        ? `${label(c)} provider will act out the problem for ${draft.minutes} min`
        : `${label(c)} channel saved`,
    );
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    saving.value = '';
  }
}
onMounted(() => {
  void load();
  timer = setInterval(() => {
    if (document.visibilityState === 'visible') void load();
  }, 5000);
});
onBeforeUnmount(() => clearInterval(timer));
</script>
<template>
  <section class="settings-card channel-health" aria-labelledby="channel-health-title">
    <h3 id="channel-health-title">Channels</h3>
    <p class="settings-help">
      Email goes through the SMTP server configured on your server; SMS uses the built-in mock
      provider, so no real text is sent. A 429 or a refused key holds the channel for every worker
      and costs messages none of their five attempts. A failing provider is held until test calls go
      through.
    </p>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
    <p v-if="anyTrouble" class="channel-trouble" role="status">
      A channel is held. Its messages keep their place and send once it recovers.
    </p>
    <div class="channel-list">
      <article v-for="c in channels" :key="c.provider" class="channel-row">
        <div class="channel-head">
          <strong>{{ label(c) }}</strong
          ><span :class="['channel-state', states[c.state].tone]"
            ><i aria-hidden="true" />{{ states[c.state].label }}</span
          >
        </div>
        <p class="channel-help">
          {{ states[c.state].help
          }}<template v-if="c.until">
            Until {{ formatDateTime(c.until, { seconds: true }) }}.</template
          >
        </p>
        <dl class="channel-stats">
          <div>
            <dt>Calls, last 10 min</dt>
            <dd>{{ c.calls_last_10m }}</dd>
          </div>
          <div>
            <dt>Accepted</dt>
            <dd>{{ c.accepted_last_10m }}</dd>
          </div>
          <div>
            <dt>Failed</dt>
            <dd>{{ c.failed_last_10m }}</dd>
          </div>
          <div>
            <dt>Held (429/401)</dt>
            <dd>{{ c.held_last_10m }}</dd>
          </div>
        </dl>
        <p v-if="c.last_problem" class="channel-problem">
          Last problem {{ formatDateTime(c.last_problem.at, { seconds: true }) }}:
          {{ problemText(c.last_problem) }}
        </p>
        <p v-if="c.simulate" class="channel-simulating" role="status">
          Simulating {{ faults.find((f) => f.value === c.simulate)?.label.toLowerCase() }} until
          {{ formatDateTime(c.simulate_until) }}. Nothing reaches the provider meanwhile.
        </p>
        <form v-if="drafts[c.provider]" class="channel-form" @submit.prevent="save(c)">
          <label
            >Calls per second<input
              v-model.number="drafts[c.provider].rate"
              type="number"
              min="1"
              max="1000"
              required /></label
          ><label
            >Simulate a provider problem<AppSelect
              v-model="drafts[c.provider].simulate"
              :options="faults" /></label
          ><label v-if="drafts[c.provider].simulate"
            >For (minutes)<input
              v-model.number="drafts[c.provider].minutes"
              type="number"
              min="1"
              max="30"
              required /></label
          ><button :disabled="saving === c.provider" :aria-label="`Save ${label(c)} channel`">
            {{ saving === c.provider ? 'Saving…' : 'Save' }}
          </button>
        </form>
      </article>
    </div>
  </section>
</template>
<style scoped>
.channel-trouble,
.channel-simulating {
  margin: 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--amber-97);
  color: var(--amber-53-text);
  font-size: 12px;
}
.channel-list {
  display: grid;
  gap: 12px;
}
.channel-row {
  display: grid;
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.channel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.channel-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
}
.channel-state i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.channel-state.good {
  color: var(--green-43-text);
}
.channel-state.good i {
  background: var(--green-54);
}
.channel-state.warn {
  color: var(--amber-53-text);
}
.channel-state.warn i {
  background: var(--amber-62);
}
.channel-state.bad {
  color: var(--red-49-text);
}
.channel-state.bad i {
  background: var(--red-56);
}
.channel-help,
.channel-problem {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.channel-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}
.channel-stats div {
  display: grid;
  gap: 2px;
}
.channel-stats dt {
  color: var(--muted);
  font-size: 11px;
}
.channel-stats dd {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.channel-form {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr) 110px auto;
  gap: 10px;
  align-items: end;
}
@media (max-width: 760px) {
  .channel-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .channel-form {
    grid-template-columns: 1fr;
  }
}
</style>
