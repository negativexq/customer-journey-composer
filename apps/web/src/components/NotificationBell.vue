<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import { errorMessage, formatDateTime } from '../lib/format';
import AppIcon from './ui/AppIcon.vue';

type Notification = {
  id: string;
  kind: string;
  severity: 'info' | 'success' | 'warning' | 'critical';
  title: string;
  body: string;
  journey_id?: string;
  created_at: string;
  unread: boolean;
};
const emit = defineEmits<{ journey: [string]; channels: [] }>();
const open = ref(false);
const unread = ref(0);
const items = ref<Notification[]>([]);
const error = ref('');
const root = ref<HTMLElement>();
let timer: ReturnType<typeof setInterval> | undefined;

async function load() {
  try {
    const result = await api<{ unread: number; items: Notification[] }>('/notifications');
    unread.value = result.unread;
    items.value = result.items;
    error.value = '';
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function toggle() {
  open.value = !open.value;
  if (!open.value) return;
  await load();
  if (unread.value > 0) {
    // Items keep their unread mark while the panel is open; the count clears.
    unread.value = 0;
    try {
      await api('/notifications/read', 'POST', {});
    } catch {
      // The count comes back on the next check if marking failed.
    }
  }
}
function choose(item: Notification) {
  open.value = false;
  if (item.journey_id) emit('journey', item.journey_id);
  else if (item.kind.startsWith('channel.')) emit('channels');
}
function outside(event: MouseEvent) {
  if (open.value && root.value && !root.value.contains(event.target as Node)) open.value = false;
}
function escape(event: KeyboardEvent) {
  if (open.value && event.key === 'Escape') open.value = false;
}
onMounted(() => {
  void load();
  timer = setInterval(() => {
    if (!open.value && document.visibilityState === 'visible') void load();
  }, 15000);
  document.addEventListener('mousedown', outside);
  document.addEventListener('keydown', escape);
});
onBeforeUnmount(() => {
  clearInterval(timer);
  document.removeEventListener('mousedown', outside);
  document.removeEventListener('keydown', escape);
});
</script>
<template>
  <div ref="root" class="notification-bell">
    <button
      class="icon-button"
      :aria-label="unread ? `Notifications, ${unread} unread` : 'Notifications'"
      :aria-expanded="open"
      title="Notifications"
      @click="toggle"
    >
      <AppIcon name="bell" :size="17" /><span v-if="unread" class="bell-count">{{
        unread > 9 ? '9+' : unread
      }}</span>
    </button>
    <section v-if="open" class="bell-panel" aria-label="Notifications">
      <header>
        <strong>Notifications</strong><small>Channel problems and what your team did</small>
      </header>
      <p v-if="error" role="alert" class="error-message">{{ error }}</p>
      <p v-else-if="!items.length" class="bell-empty">
        Nothing yet. Publishing, pausing and channel problems show up here.
      </p>
      <ul v-else>
        <li v-for="item in items" :key="item.id">
          <button
            type="button"
            :class="['bell-item', item.severity, { unread: item.unread }]"
            :disabled="!item.journey_id && !item.kind.startsWith('channel.')"
            @click="choose(item)"
          >
            <i aria-hidden="true" />
            <span
              ><strong>{{ item.title }}</strong
              ><small v-if="item.body">{{ item.body }}</small
              ><time :datetime="item.created_at">{{ formatDateTime(item.created_at) }}</time></span
            >
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>
<style scoped>
.notification-bell {
  position: relative;
}
.notification-bell > .icon-button {
  position: relative;
}
.bell-count {
  position: absolute;
  top: 1px;
  right: 0;
  min-width: 15px;
  height: 15px;
  padding: 0 4px;
  border-radius: 8px;
  background: var(--red-56);
  color: var(--on-solid);
  font-size: 10px;
  font-weight: 700;
  line-height: 15px;
  text-align: center;
}
.bell-panel {
  position: absolute;
  z-index: 40;
  top: calc(100% + 8px);
  right: 0;
  width: min(380px, calc(100vw - 24px));
  max-height: min(520px, calc(100vh - 90px));
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  box-shadow: 0 12px 32px rgb(0 0 0 / 14%);
}
.bell-panel header {
  display: grid;
  gap: 2px;
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--border);
}
.bell-panel header small,
.bell-empty {
  color: var(--muted);
  font-size: 12px;
}
.bell-empty {
  margin: 0;
  padding: 20px 16px;
}
.bell-panel ul {
  margin: 0;
  padding: 6px;
  list-style: none;
}
.bell-item {
  display: flex;
  gap: 10px;
  width: 100%;
  min-height: 0;
  padding: 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  text-align: left;
  align-items: flex-start;
  justify-content: flex-start;
  font-weight: 400;
}
.bell-item:not(:disabled):hover {
  background: var(--gray-96);
}
.bell-item:disabled {
  cursor: default;
  opacity: 1;
}
.bell-item.unread {
  background: var(--accent-soft);
}
.bell-item i {
  flex: none;
  width: 8px;
  height: 8px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--gray-74);
}
.bell-item.success i {
  background: var(--green-54);
}
.bell-item.warning i {
  background: var(--amber-62);
}
.bell-item.critical i {
  background: var(--red-56);
}
.bell-item span {
  display: grid;
  gap: 3px;
  min-width: 0;
}
.bell-item strong {
  font-size: 13px;
  font-weight: 600;
}
.bell-item small {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.45;
}
.bell-item time {
  color: var(--muted);
  font-size: 12px;
}
</style>
