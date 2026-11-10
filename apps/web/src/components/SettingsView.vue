<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../lib/api';
const keys = ref<Record<string, unknown>[]>([]);
const name = ref('');
const token = ref('');
const error = ref('');
async function load() {
  try {
    keys.value = await api('/settings/api-keys');
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
onMounted(load);
</script>
<template>
  <div class="page-heading"><h1>Settings</h1></div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
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
