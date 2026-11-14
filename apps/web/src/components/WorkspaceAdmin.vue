<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import EmptyState from './ui/EmptyState.vue';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import { api } from '../lib/api';
import { initials } from '../lib/customer-journey';
import { errorMessage, formatDateTime, plural } from '../lib/format';
import { toast } from '../lib/toast';

// People, the record of what they changed, and the health of the services behind the workspace.
defineProps<{ section: 'team' | 'audit' | 'system' }>();
type User = { id: string; name: string; email: string; role: string };
type Audit = {
  id: string;
  action: string;
  actor_id: string;
  resource_id: string;
  created_at: string;
  details: unknown;
};
type SystemStatus = {
  analytics_processed_at?: string | null;
  failed_jobs?: unknown[];
  failed_or_unknown_deliveries?: unknown[];
  oldest_metric_delta?: string | null;
  pool?: { acquired: number; idle: number; max: number; total: number };
  process?: { goroutines: number; heap_alloc_bytes: number };
};
const roles = ['Admin', 'Editor', 'Viewer'];
const roleHelp: Record<string, string> = {
  Admin: 'Everything, including settings and people',
  Editor: 'Builds, tests and publishes journeys',
  Viewer: 'Reads journeys and results',
};
const users = ref<User[]>([]);
const audits = ref<Audit[]>([]);
const system = ref<SystemStatus>();
const name = ref('');
const email = ref('');
const password = ref('');
const role = ref('Viewer');
const adding = ref(false);
const error = ref('');
const busy = ref(false);
const loaded = ref(false);

async function refresh() {
  error.value = '';
  try {
    [users.value, audits.value, system.value] = await Promise.all([
      api<User[]>('/users'),
      api<Audit[]>('/audit-logs'),
      api<SystemStatus>('/system/status'),
    ]);
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    loaded.value = true;
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
    adding.value = false;
    toast('Member added');
    await refresh();
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    busy.value = false;
  }
}
async function change(user: User, next: string) {
  if (next === user.role) return;
  busy.value = true;
  error.value = '';
  try {
    await api(`/users/${user.id}/role`, 'PUT', { role: next });
    toast(`${user.name} is now ${next}`);
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    // Reload either way so a refused change does not leave the wrong role on screen.
    await refresh();
    busy.value = false;
  }
}
const actions: Record<string, string> = {
  'journey.publish': 'Journey published',
  'journey.creation_approved': 'Journey proposal approved',
  'user.create': 'Member added',
  'user.role': 'Member role changed',
  'api_key.create': 'API key created',
  'api_key.revoke': 'API key revoked',
  'data_catalog.updated': 'Workspace data updated',
  'workspace.messaging_policy': 'Messaging policy changed',
  'demo.workspace_reset': 'Demo workspace reset',
  'ai.generation': 'AI generation requested',
  'test_scenario.requirement': 'Test requirement recorded',
};
const actionLabel = (action: string) => {
  const text = actions[action] || action.replace(/[._]/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
};
const actor = (id: string) => users.value.find((user) => user.id === id)?.name || 'System';
const hasDetails = (details: unknown) =>
  !!details && typeof details === 'object' && Object.keys(details).length > 0;
const megabytes = (bytes?: number) => (bytes ? (bytes / 1048576).toFixed(1) + ' MB' : '—');
const problems = computed(
  () =>
    (system.value?.failed_jobs?.length || 0) +
    (system.value?.failed_or_unknown_deliveries?.length || 0),
);
onMounted(() => void refresh());
</script>
<template>
  <p v-if="error" class="inline-alert error-alert" role="alert">{{ error }}</p>
  <template v-if="section === 'team'">
    <header class="settings-heading with-action">
      <div>
        <h2>Team</h2>
        <p>Who can open this workspace and what they may change.</p>
      </div>
      <button v-if="!adding" class="primary" type="button" @click="adding = true">
        <AppIcon name="plus" :size="15" />Add member
      </button>
    </header>
    <form v-if="adding" class="settings-card" @submit.prevent="create">
      <h3>New member</h3>
      <p class="settings-help">
        Share the initial password with them yourself; it is never shown again or written to the
        audit log.
      </p>
      <div class="settings-fields">
        <label>User name<input v-model="name" required maxlength="120" /></label
        ><label>User email<input v-model="email" type="email" required maxlength="254" /></label>
        <div class="settings-field">
          <label
            >Initial password<input
              v-model="password"
              type="password"
              minlength="12"
              maxlength="72"
              required
              autocomplete="new-password"
              aria-describedby="member-password-hint"
          /></label>
          <small id="member-password-hint">At least 12 characters.</small>
        </div>
        <div class="settings-field">
          <label for="member-role">New user role</label>
          <select id="member-role" v-model="role" aria-describedby="member-role-hint">
            <option v-for="item in roles" :key="item">{{ item }}</option>
          </select>
          <small id="member-role-hint">{{ roleHelp[role] }}</small>
        </div>
      </div>
      <div class="settings-actions">
        <button class="primary" :disabled="busy" type="submit">Create user</button
        ><button type="button" :disabled="busy" @click="adding = false">Cancel</button>
      </div>
    </form>
    <section class="settings-card">
      <h3>
        Members <span class="settings-count">{{ users.length }}</span>
      </h3>
      <SkeletonBlock v-if="!loaded" label="Loading members…" :rows="3" />
      <ul v-else class="settings-list">
        <li v-for="user in users" :key="user.id">
          <span class="member-avatar" aria-hidden="true">{{ initials(user.name) }}</span>
          <div>
            <strong>{{ user.name }}</strong
            ><small>{{ user.email }}</small>
          </div>
          <select
            :value="user.role"
            :disabled="busy"
            :aria-label="`Role for ${user.name}`"
            @change="change(user, ($event.target as HTMLSelectElement).value)"
          >
            <option v-for="item in roles" :key="item">{{ item }}</option>
          </select>
        </li>
      </ul>
      <p class="settings-help">The workspace always keeps at least one Admin.</p>
    </section>
  </template>
  <template v-else-if="section === 'audit'">
    <header class="settings-heading with-action">
      <div>
        <h2>Audit log</h2>
        <p>Changes people made to this workspace, newest first.</p>
      </div>
      <button type="button" @click="refresh"><AppIcon name="refresh" :size="15" />Refresh</button>
    </header>
    <section class="settings-card">
      <SkeletonBlock v-if="!loaded" label="Loading audit log…" :rows="5" />
      <EmptyState v-else-if="!audits.length" icon="history" title="Nothing recorded yet" compact>
        Publishing, role changes and key changes are listed here.
      </EmptyState>
      <ul v-else class="audit-list">
        <li v-for="entry in audits" :key="entry.id">
          <div class="audit-row">
            <strong>{{ actionLabel(entry.action) }}</strong>
            <span>{{ actor(entry.actor_id) }}</span>
            <time :datetime="entry.created_at">{{
              formatDateTime(entry.created_at, { seconds: true })
            }}</time>
          </div>
          <details v-if="hasDetails(entry.details) || entry.resource_id">
            <summary>Details</summary>
            <p v-if="entry.resource_id">
              Resource <code>{{ entry.resource_id }}</code> · action <code>{{ entry.action }}</code>
            </p>
            <pre v-if="hasDetails(entry.details)">{{ JSON.stringify(entry.details, null, 2) }}</pre>
          </details>
        </li>
      </ul>
    </section>
  </template>
  <template v-else>
    <header class="settings-heading with-action">
      <div>
        <h2>System</h2>
        <p>Health of the workers and services behind this workspace.</p>
      </div>
      <button type="button" @click="refresh"><AppIcon name="refresh" :size="15" />Refresh</button>
    </header>
    <SkeletonBlock v-if="!loaded" label="Loading system status…" variant="cards" :rows="4" />
    <template v-else-if="system">
      <div class="settings-status" :class="problems ? 'off' : 'on'">
        <span class="small-dot" />{{
          problems
            ? `${plural(problems, 'item')} need attention`
            : 'No failed jobs or undelivered messages'
        }}
      </div>
      <dl class="system-grid">
        <div>
          <dt>Failed jobs</dt>
          <dd>{{ system.failed_jobs?.length || 0 }}</dd>
          <small>Journey steps that stopped after retries</small>
        </div>
        <div>
          <dt>Failed or unknown deliveries</dt>
          <dd>{{ system.failed_or_unknown_deliveries?.length || 0 }}</dd>
          <small>Messages without a confirmed outcome</small>
        </div>
        <div>
          <dt>Analytics updated</dt>
          <dd class="text">{{ formatDateTime(system.analytics_processed_at || '') }}</dd>
          <small>{{
            system.oldest_metric_delta ? 'Counters are catching up' : 'Counters are up to date'
          }}</small>
        </div>
        <div v-if="system.pool">
          <dt>Database connections</dt>
          <dd>{{ system.pool.acquired }} / {{ system.pool.max }}</dd>
          <small>In use of the pool · {{ system.pool.idle }} idle</small>
        </div>
        <div v-if="system.process">
          <dt>API memory</dt>
          <dd>{{ megabytes(system.process.heap_alloc_bytes) }}</dd>
          <small>{{ system.process.goroutines }} goroutines</small>
        </div>
      </dl>
      <details class="settings-card system-raw">
        <summary>Technical details</summary>
        <pre>{{ JSON.stringify(system, null, 2) }}</pre>
      </details>
    </template>
  </template>
</template>
<style scoped>
.member-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-text);
  font-size: 12px;
  font-weight: 600;
}
.settings-list select {
  width: auto;
  min-width: 110px;
}
.audit-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.audit-list li {
  padding: 12px 0;
  border-top: 1px solid var(--border);
}
.audit-list li:first-child {
  border-top: 0;
  padding-top: 0;
}
.audit-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 160px 170px;
  gap: 12px;
  align-items: baseline;
  font-size: 13px;
}
.audit-row strong {
  font-weight: 500;
}
.audit-row span,
.audit-row time {
  color: var(--muted);
  font-size: 12px;
}
.audit-row time {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
details {
  margin-top: 6px;
  font-size: 12px;
  color: var(--muted);
}
summary {
  cursor: pointer;
}
details p {
  margin: 8px 0;
  overflow-wrap: anywhere;
}
pre {
  margin: 8px 0 0;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--page);
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 360px;
  overflow: auto;
}
.system-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  margin: 0;
}
.system-grid div {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
.system-grid dt {
  font-size: 12px;
  color: var(--muted);
}
.system-grid dd {
  margin: 6px 0 4px;
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.system-grid dd.text {
  font-size: 15px;
}
.system-grid small {
  font-size: 12px;
  color: var(--muted);
}
.system-raw summary {
  font-size: 13px;
  color: var(--text);
}
@media (max-width: 700px) {
  .audit-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 2px;
  }
  .audit-row time {
    text-align: left;
  }
}
</style>
