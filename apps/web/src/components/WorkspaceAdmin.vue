<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../lib/api';
const users = ref<{ id: string; name: string; email: string; role: string }[]>([]);
const audits = ref<
  {
    id: string;
    action: string;
    actor_id: string;
    resource_id: string;
    created_at: string;
    details: unknown;
  }[]
>([]);
const name = ref('');
const email = ref('');
const password = ref('');
const role = ref('Viewer');
const error = ref('');
const busy = ref(false);
const system = ref<unknown>();
async function refresh() {
  try {
    [users.value, audits.value, system.value] = await Promise.all([
      api<typeof users.value>('/users'),
      api<typeof audits.value>('/audit-logs'),
      api('/system/status'),
    ]);
  } catch (cause) {
    error.value = String(cause);
  }
}
async function create() {
  busy.value = true;
  error.value = '';
  try {
    await api('/users', 'POST', {
      name: name.value,
      email: email.value,
      password: password.value,
      role: role.value,
    });
    password.value = '';
    name.value = '';
    email.value = '';
    await refresh();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
async function change(id: string, next: string) {
  busy.value = true;
  error.value = '';
  try {
    await api(`/users/${id}/role`, 'PUT', { role: next });
    await refresh();
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
onMounted(() => void refresh());
</script>
<template>
  <section class="panel">
    <h2>Workspace users</h2>
    <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    <form class="editor-form" @submit.prevent="create">
      <label>User name<input v-model="name" required maxlength="120" /></label
      ><label>User email<input v-model="email" type="email" required maxlength="254" /></label
      ><label
        >Initial password<input
          v-model="password"
          type="password"
          minlength="12"
          maxlength="72"
          required
          autocomplete="new-password" /></label
      ><label
        >New user role<select v-model="role">
          <option>Viewer</option>
          <option>Editor</option>
          <option>Admin</option>
        </select></label
      ><button :disabled="busy" type="submit">Create user</button>
    </form>
    <div v-for="u in users" :key="u.id" class="list-row">
      <span>{{ u.name }} · {{ u.email }} · {{ u.role }}</span>
      <div>
        <button
          v-for="r in ['Admin', 'Editor', 'Viewer'].filter((r) => r !== u.role)"
          :key="r"
          :disabled="busy"
          @click="change(u.id, r)"
        >
          Set {{ r }}
        </button>
      </div>
    </div>
    <p>At least one Admin is retained. Passwords are never shown in user lists or audit records.</p>
  </section>
  <section class="panel">
    <h2>Audit logs</h2>
    <button @click="refresh">Refresh administration</button>
    <details v-for="a in audits" :key="a.id">
      <summary>{{ a.created_at }} · {{ a.action }}</summary>
      <p>Actor {{ a.actor_id }} · resource {{ a.resource_id }}</p>
      <pre>{{ JSON.stringify(a.details, null, 2) }}</pre>
    </details>
  </section>
  <section class="panel">
    <h2>System status</h2>
    <pre>{{ JSON.stringify(system, null, 2) }}</pre>
  </section>
</template>
<style scoped>
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 600px;
  overflow: auto;
}
.list-row {
  flex-wrap: wrap;
}
</style>
