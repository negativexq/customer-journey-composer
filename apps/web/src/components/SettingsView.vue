<script setup lang="ts">
import { toast } from '../lib/toast';
import { errorMessage } from '../lib/format';
import DataCatalog from './DataCatalog.vue';
import WorkspaceAdmin from './WorkspaceAdmin.vue';
import ConfirmDialog from './ui/ConfirmDialog.vue';
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import EmptyState from './ui/EmptyState.vue';
import { settingsSections, type SettingsSection } from '../lib/navigation';
import { api } from '../lib/api';
const props = defineProps<{ section?: SettingsSection }>();
const emit = defineEmits<{ section: [SettingsSection] }>();
// One screen per concern, grouped by who usually needs it.
const groups: { label: string; items: { id: SettingsSection; label: string; icon: string }[] }[] = [
  {
    label: 'Workspace',
    items: [
      { id: 'messaging', label: 'Messaging', icon: 'email' },
      { id: 'data', label: 'Workspace data', icon: 'templates' },
      { id: 'team', label: 'Team', icon: 'users' },
    ],
  },
  {
    label: 'Developers',
    items: [
      { id: 'api-keys', label: 'API keys', icon: 'lock' },
      { id: 'ai', label: 'AI providers', icon: 'sparkles' },
    ],
  },
  {
    label: 'Administration',
    items: [
      { id: 'audit', label: 'Audit log', icon: 'history' },
      { id: 'system', label: 'System', icon: 'shield' },
    ],
  },
];
const current = computed(() => props.section || settingsSections[0]);
const saving = ref(false);
const nav = ref<HTMLElement>();
// On a phone the sections are one scrolling row; keep the open one in sight.
async function revealCurrent() {
  await nextTick();
  const active = nav.value?.querySelector<HTMLElement>('[aria-current="page"]');
  if (nav.value && active && nav.value.scrollWidth > nav.value.clientWidth)
    nav.value.scrollLeft = active.offsetLeft - (nav.value.clientWidth - active.offsetWidth) / 2;
}
watch(current, revealCurrent);
const keys = ref<{ id: string; name: string; prefix: string; revoked_at?: string | null }[]>([]);
const name = ref('');
const token = ref('');
const error = ref('');
const limit = ref(3);
const hours = ref(24);
const allowlist = ref('');
const copied = ref(false);
async function copyToken() {
  try {
    await navigator.clipboard.writeText(token.value);
    copied.value = true;
    toast('API key copied');
  } catch {
    // Clipboard access can be refused; the key stays visible to copy by hand.
  }
}
const ai = ref<{
  enabled: boolean;
  providers: Record<string, { configured: boolean; model: string }>;
}>();
async function load() {
  try {
    const [keyList, policy] = await Promise.all([
      api<typeof keys.value>('/settings/api-keys'),
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
    error.value = errorMessage(cause);
  }
}
async function create() {
  try {
    const result = await api<{ token: string }>('/settings/api-keys', 'POST', { name: name.value });
    token.value = result.token;
    name.value = '';
    toast('API key created');
    await load();
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
const revoking = ref<{ id: string; name: string }>();
async function confirmRevoke() {
  const key = revoking.value;
  revoking.value = undefined;
  if (key) await revoke(key.id);
}
async function revoke(id: string) {
  try {
    await api(`/settings/api-keys/${id}`, 'DELETE');
    toast('API key revoked');
    await load();
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function savePolicy() {
  error.value = '';
  saving.value = true;
  try {
    await api('/settings/messaging', 'PUT', {
      frequency_limit: limit.value,
      frequency_window_seconds: Math.round(hours.value * 3600),
      test_allowlist: allowlist.value
        .split(/[\n,]/)
        .map((v) => v.trim())
        .filter(Boolean),
    });
    toast('Messaging policy saved');
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    saving.value = false;
  }
}
onMounted(() => {
  void load();
  void revealCurrent();
});
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">WORKSPACE</p>
      <h1>Settings</h1>
      <p class="page-description">
        Sending rules, your data, people and access for this workspace.
      </p>
    </div>
  </div>
  <div class="settings-layout">
    <nav ref="nav" class="settings-nav" aria-label="Settings sections">
      <div v-for="group in groups" :key="group.label" class="settings-nav-group">
        <p>{{ group.label }}</p>
        <button
          v-for="item in group.items"
          :key="item.id"
          type="button"
          :aria-current="current === item.id ? 'page' : undefined"
          @click="emit('section', item.id)"
        >
          <AppIcon :name="item.icon" :size="16" />{{ item.label }}
        </button>
      </div>
    </nav>
    <div class="settings-content">
      <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
      <template v-if="current === 'messaging'">
        <header class="settings-heading">
          <h2>Messaging</h2>
          <p>How often a customer can hear from you, and who may receive test sends.</p>
        </header>
        <form class="settings-card" @submit.prevent="savePolicy">
          <h3>Sending limit</h3>
          <p class="settings-help">
            A customer receives at most this many messages inside the window, across every journey.
            Pending messages reserve a slot; failed and blocked ones do not count.
          </p>
          <div class="settings-fields">
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
          <h3>Test recipients</h3>
          <p class="settings-help">
            Send Test only delivers to these addresses and numbers. Test sends do not count toward
            the limit above.
          </p>
          <label
            >Test recipient allowlist<textarea
              v-model="allowlist"
              rows="4"
              placeholder="One authorized email address or E.164 phone number per line"
            />
          </label>
          <div class="settings-actions">
            <button class="primary" :disabled="saving">
              {{ saving ? 'Saving…' : 'Save messaging policy' }}
            </button>
          </div>
        </form>
        <section class="settings-card">
          <h3>Channels</h3>
          <dl class="settings-rows">
            <div>
              <dt>Email</dt>
              <dd>Sent through the SMTP server configured on your server.</dd>
            </div>
            <div>
              <dt>SMS</dt>
              <dd>Uses the built-in mock provider; no real text messages are sent.</dd>
            </div>
          </dl>
        </section>
      </template>
      <DataCatalog v-else-if="current === 'data'" />
      <WorkspaceAdmin
        v-else-if="current === 'team' || current === 'audit' || current === 'system'"
        :section="current"
      />
      <template v-else-if="current === 'api-keys'">
        <header class="settings-heading">
          <h2>API keys</h2>
          <p>
            A key lets your systems send events into this workspace and nothing else. A new key is
            shown once, so store it before leaving this page.
          </p>
        </header>
        <section class="settings-card">
          <form class="settings-inline" @submit.prevent="create">
            <label
              >Key name<input
                v-model="name"
                required
                maxlength="120"
                placeholder="Storefront backend" /></label
            ><button class="primary">Create API key</button>
          </form>
          <div v-if="token" class="key-banner">
            <strong>Copy this key now. It will not be shown again.</strong>
            <div class="settings-inline">
              <label
                >New API key<input
                  :value="token"
                  readonly
                  @focus="($event.target as HTMLInputElement).select()" /></label
              ><button type="button" @click="copyToken">{{ copied ? 'Copied' : 'Copy' }}</button
              ><button
                type="button"
                @click="
                  token = '';
                  copied = false;
                "
              >
                Hide key
              </button>
            </div>
          </div>
        </section>
        <section class="settings-card">
          <h3>Keys in this workspace</h3>
          <EmptyState v-if="!keys.length" icon="lock" title="No API keys yet" compact>
            Create one to start sending events from your systems.
          </EmptyState>
          <ul v-else class="settings-list">
            <li v-for="key in keys" :key="key.id">
              <div>
                <strong>{{ key.name }}</strong
                ><small>{{ key.prefix }}…</small>
              </div>
              <span class="settings-tag" :class="key.revoked_at ? 'off' : 'on'">{{
                key.revoked_at ? 'Revoked' : 'Active'
              }}</span>
              <button v-if="!key.revoked_at" type="button" @click="revoking = key">Revoke</button>
            </li>
          </ul>
        </section>
        <ConfirmDialog
          :model-value="!!revoking"
          title="Revoke this API key?"
          confirm-label="Revoke key"
          danger
          @update:model-value="revoking = undefined"
          @confirm="confirmRevoke"
        >
          <p>
            <strong>{{ revoking?.name }}</strong> stops working immediately. Systems that still use
            it will have their events rejected. This cannot be undone.
          </p>
        </ConfirmDialog>
      </template>
      <template v-else-if="current === 'ai'">
        <header class="settings-heading">
          <h2>AI providers</h2>
          <p>
            Credentials and model IDs are set on the server. Keys are never sent to the browser.
          </p>
        </header>
        <section v-if="ai" class="settings-card">
          <div class="settings-status" :class="ai.enabled ? 'on' : 'off'">
            <span class="small-dot" />{{
              ai.enabled ? 'Live generation is enabled' : 'Live generation is disabled'
            }}
          </div>
          <ul class="settings-list">
            <li v-for="(provider, providerName) in ai.providers" :key="providerName">
              <div>
                <strong>{{ providerName === 'claude' ? 'Claude' : 'OpenAI' }}</strong
                ><small>{{
                  provider.configured ? provider.model : 'No credentials on the server'
                }}</small>
              </div>
              <span class="settings-tag" :class="provider.configured ? 'on' : 'off'">{{
                provider.configured ? 'Configured' : 'Not configured'
              }}</span>
            </li>
          </ul>
        </section>
      </template>
    </div>
  </div>
</template>
<style scoped>
.settings-layout {
  display: grid;
  grid-template-columns: 208px minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}
.settings-nav {
  position: sticky;
  top: 16px;
  display: grid;
  gap: 16px;
}
.settings-nav-group {
  display: grid;
  gap: 2px;
}
.settings-nav p {
  margin: 0 0 4px 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}
.settings-nav button {
  justify-content: flex-start;
  gap: 10px;
  width: 100%;
  min-height: 34px;
  padding: 6px 10px;
  border-color: transparent;
  background: none;
  color: var(--text);
  font-size: 13px;
}
.settings-nav button svg {
  color: var(--muted);
}
.settings-nav button:hover {
  background: var(--surface);
  border-color: transparent;
}
.settings-nav button[aria-current='page'] {
  background: var(--accent-soft);
  color: var(--accent-text);
  font-weight: 600;
}
.settings-nav button[aria-current='page'] svg {
  color: var(--accent-text);
}
.settings-content {
  display: grid;
  gap: 16px;
  min-width: 0;
  max-width: 880px;
}
.key-banner {
  display: grid;
  gap: 10px;
  margin-top: 16px;
  padding: 14px;
  border: 1px solid var(--amber-90);
  border-radius: 10px;
  background: var(--amber-99);
}
.key-banner strong {
  font-size: 13px;
  color: var(--amber-51-text);
}
@media (max-width: 900px) {
  .settings-layout {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  /* Sections become one scrolling row of tabs. */
  .settings-nav {
    position: static;
    display: flex;
    gap: 4px;
    overflow-x: auto;
    scrollbar-width: none;
    border-bottom: 1px solid var(--border);
    padding-bottom: 6px;
  }
  .settings-nav-group {
    display: flex;
    gap: 4px;
  }
  .settings-nav p {
    display: none;
  }
  .settings-nav button {
    width: auto;
    min-height: 44px;
    white-space: nowrap;
  }
}
</style>
