<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../lib/api';
const keys = ref<Record<string, unknown>[]>([]);
const name = ref('');
const token = ref('');
const error = ref('');
const limit = ref(3);
const hours = ref(24);
const allowlist = ref('');
const notice = ref('');
const ai = ref<{
  enabled: boolean;
  providers: Record<string, { configured: boolean; model: string }>;
}>();
async function load() {
  try {
    const [keyList, policy] = await Promise.all([
      api<Record<string, unknown>[]>('/settings/api-keys'),
      api<{ frequency_limit: number; frequency_window_seconds: number; test_allowlist: string[] }>(
        '/settings/messaging',
      ),
    ]);
    keys.value = keyList;
    ai.value = await api('/settings/ai');
    limit.value = policy.frequency_limit;
    hours.value = policy.frequency_window_seconds / 3600;
    allowlist.value = policy.test_allowlist.join('\n');
  } catch (cause) {
    error.value = String(cause);
  }
}
async function create() {
  try {
    const result = await api<{ token: string }>('/settings/api-keys', 'POST', { name: name.value });
    token.value = result.token;
    name.value = '';
    await load();
  } catch (cause) {
    error.value = String(cause);
  }
}
async function revoke(id: unknown) {
  try {
    await api(`/settings/api-keys/${id}`, 'DELETE');
    await load();
  } catch (cause) {
    error.value = String(cause);
  }
}
async function savePolicy() {
  error.value = '';
  try {
    await api('/settings/messaging', 'PUT', {
      frequency_limit: limit.value,
      frequency_window_seconds: Math.round(hours.value * 3600),
      test_allowlist: allowlist.value
        .split(/[\n,]/)
        .map((v) => v.trim())
        .filter(Boolean),
    });
    notice.value = 'Messaging policy saved';
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(load);
</script>
<template>
  <div class="page-heading"><h1>Settings</h1></div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <p role="status">{{ notice }}</p>
  <section class="panel">
    <h2>Messaging policy</h2>
    <p>Email uses your server's SMTP configuration. SMS uses the mock provider.</p>
    <form class="editor-form" @submit.prevent="savePolicy">
      <div class="toolbar">
        <label
          >Maximum messages per customer<input
            v-model.number="limit"
            type="number"
            min="1"
            max="1000"
            required /></label
        ><label
          >Frequency window (hours)<input
            v-model.number="hours"
            type="number"
            min="1"
            max="720"
            required
        /></label>
      </div>
      <label
        >Test recipient allowlist<textarea
          v-model="allowlist"
          rows="5"
          placeholder="One authorized email address or E.164 phone number per line"
        /></label
      ><small
        >Pending messages reserve a slot. Failed/blocked messages do not count as sends. Test sends
        are isolated from the live cap.</small
      ><button class="primary">Save messaging policy</button>
    </form>
  </section>
  <section v-if="ai" class="panel">
    <h2>AI providers</h2>
    <p>
      {{ ai.enabled ? 'Live generation enabled' : 'Live generation disabled' }}. Credentials and
      model IDs are configured on the server; API keys are never sent to the browser.
    </p>
    <div v-for="(provider, providerName) in ai.providers" :key="providerName" class="list-row">
      <strong>{{ providerName === 'claude' ? 'Claude' : 'OpenAI' }}</strong
      ><span>{{ provider.configured ? provider.model : 'Not configured' }}</span>
    </div>
  </section>
  <section class="panel">
    <h2>Event ingestion API keys</h2>
    <p>
      Keys can only ingest events into this workspace. Store new keys securely; they are shown once.
    </p>
    <form class="inline-form" @submit.prevent="create">
      <label>Key name<input v-model="name" required maxlength="120" /></label
      ><button class="primary">Create API key</button>
    </form>
    <div v-if="token" class="key-reveal">
      <label>New API key<input :value="token" readonly /></label
      ><button @click="token = ''">Hide key</button>
    </div>
    <div v-for="key in keys" :key="String(key.id)" class="list-row">
      <span
        >{{ key.name }}
        <small>{{ key.prefix }}… · {{ key.revoked_at ? 'Revoked' : 'Active' }}</small></span
      ><button v-if="!key.revoked_at" @click="revoke(key.id)">Revoke</button>
    </div>
  </section>
</template>
