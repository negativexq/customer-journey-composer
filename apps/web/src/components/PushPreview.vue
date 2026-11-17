<script setup lang="ts">
import { computed, inject, ref, watch, type Ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import SegmentedControl from './ui/SegmentedControl.vue';

// A push notification as the customer's phone shows it: dropping in over the screen they are
// using, or waiting on the lock screen.
const props = withDefaults(
  defineProps<{
    title?: string;
    body?: string;
    url?: string;
    // The sending app's name, as the notification header shows it; the workspace by default.
    app?: string;
    // When the notification arrived; the lock screen clock and date follow it.
    at?: string;
  }>(),
  { title: '', body: '', url: '', app: '', at: '' },
);
const view = defineModel<'banner' | 'lock'>('view', { default: 'banner' });
// Changing the content replays the drop, so an edit shows how the banner arrives.
const replay = ref(0);
watch(
  () => [props.title, props.body, view.value],
  () => replay.value++,
);
const when = computed(() => (props.at ? new Date(props.at) : new Date()));
const clock = computed(() =>
  when.value.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' }),
);
const date = computed(() =>
  when.value.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' }),
);
const host = computed(() => {
  if (!URL.canParse(props.url)) return '';
  const target = new URL(props.url);
  return target.host + (target.pathname === '/' ? '' : target.pathname);
});
const workspace = inject<Ref<string> | undefined>('workspace-name', undefined);
const app = computed(() => props.app.trim() || workspace?.value.trim() || 'Your app');
const initial = computed(() => app.value[0].toUpperCase());
</script>
<template>
  <div class="push-preview">
    <SegmentedControl
      v-model="view"
      class="push-views"
      label="Notification view"
      hide-label
      :options="[
        { value: 'banner', label: 'Banner' },
        { value: 'lock', label: 'Lock screen', icon: 'lock' },
      ]"
    />
    <div class="push-phone" :class="view">
      <div class="push-status" aria-hidden="true">
        <span>{{ view === 'lock' ? '' : clock }}</span
        ><span class="push-island" /><span class="push-signal"><i /><i /><i /><i /></span>
      </div>
      <template v-if="view === 'lock'">
        <div class="push-lock-head" aria-hidden="true">
          <AppIcon name="lock" :size="14" />
          <span class="push-date">{{ date }}</span>
          <span class="push-clock">{{ clock }}</span>
        </div>
        <div class="push-stack">
          <article :key="replay" class="push-card rise" aria-label="Push notification">
            <span class="push-app-icon" aria-hidden="true">{{ initial }}</span>
            <div class="push-card-body">
              <header>
                <strong>{{ title || 'Notification title' }}</strong
                ><time>now</time>
              </header>
              <p>{{ body || 'The notification text appears here.' }}</p>
              <small>{{ app }}</small>
            </div>
          </article>
        </div>
        <div class="push-lock-actions" aria-hidden="true">
          <span><AppIcon name="sun" :size="16" /></span
          ><span><AppIcon name="zap" :size="16" /></span>
        </div>
      </template>
      <template v-else>
        <!-- A home screen behind the banner: rows of app icons, nothing readable. -->
        <div class="push-home" aria-hidden="true">
          <i v-for="n in 20" :key="n" />
        </div>
        <article :key="replay" class="push-card drop" aria-label="Push notification">
          <span class="push-app-icon" aria-hidden="true">{{ initial }}</span>
          <div class="push-card-body">
            <header>
              <small>{{ app }}</small
              ><time>now</time>
            </header>
            <strong>{{ title || 'Notification title' }}</strong>
            <p>{{ body || 'The notification text appears here.' }}</p>
          </div>
        </article>
        <div class="push-dock" aria-hidden="true"><i /><i /><i /><i /></div>
      </template>
      <span class="push-home-bar" aria-hidden="true" />
    </div>
    <small class="push-meta">
      <template v-if="host">Tapping opens {{ host }}</template>
      <template v-else>Tapping opens the app</template>
    </small>
  </div>
</template>
<style scoped>
.push-preview {
  display: grid;
  justify-items: center;
  gap: 12px;
  padding: 20px 12px 24px;
  border-radius: 12px;
  background: var(--gray-96);
}
.push-phone {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(100%, 320px);
  height: 600px;
  padding: 10px 12px 14px;
  border: 9px solid var(--gray-25);
  border-radius: 46px;
  background: linear-gradient(170deg, var(--phone-wallpaper-top), var(--phone-wallpaper-bottom));
  box-shadow:
    0 0 0 1px var(--gray-74),
    0 24px 48px -16px #17213b4d;
  overflow: hidden;
}
.push-status {
  /* Three columns keep the island centred whether or not the clock shows. */
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  min-height: 28px;
  padding: 2px 14px 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--phone-clock);
}
.push-island {
  width: 86px;
  height: 24px;
  border-radius: 999px;
  background: var(--gray-25);
}
.push-signal {
  justify-self: end;
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 11px;
}
.push-signal i {
  width: 3px;
  border-radius: 1px;
  background: var(--phone-clock);
}
.push-signal i:nth-child(1) {
  height: 4px;
}
.push-signal i:nth-child(2) {
  height: 6px;
}
.push-signal i:nth-child(3) {
  height: 8px;
}
.push-signal i:nth-child(4) {
  height: 11px;
}
.push-lock-head {
  display: grid;
  justify-items: center;
  gap: 2px;
  padding: 6px 0 22px;
  color: var(--phone-clock);
}
.push-date {
  margin-top: 6px;
  font-size: 15px;
  font-weight: 600;
}
.push-clock {
  font-size: 72px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -1px;
  font-variant-numeric: tabular-nums;
}
.push-stack {
  flex: 1;
  display: grid;
  align-content: start;
  padding: 0 2px;
}
.push-card {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  gap: 10px;
  padding: 11px 12px;
  border-radius: 20px;
  background: var(--phone-notification);
  color: var(--phone-notification-text);
  box-shadow: 0 8px 24px -10px #0b0f1a59;
  backdrop-filter: blur(18px) saturate(1.4);
}
.push-app-icon {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: var(--accent);
  color: var(--on-solid);
  font-size: 17px;
  font-weight: 700;
}
.push-card-body {
  display: grid;
  gap: 1px;
  min-width: 0;
}
.push-card-body header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}
.push-card strong {
  display: -webkit-box;
  overflow: hidden;
  font-size: 14px;
  line-height: 1.3;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}
.push-card p {
  display: -webkit-box;
  margin: 0;
  color: inherit;
  overflow: hidden;
  font-size: 14px;
  line-height: 1.35;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  line-clamp: 4;
}
.push-card small,
.push-card time {
  font-size: 12px;
  opacity: 0.7;
}
.push-card small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.push-card time {
  flex: none;
}
.push-phone.banner .push-card {
  position: absolute;
  top: 46px;
  right: 10px;
  left: 10px;
  z-index: 1;
}
.push-phone.banner .push-card small {
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.push-home {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 22px 18px;
  padding: 26px 10px 0;
  filter: blur(1.5px);
  opacity: 0.55;
}
.push-home i,
.push-dock i {
  aspect-ratio: 1;
  border-radius: 14px;
  background: var(--phone-notification);
}
.push-dock {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
  margin: auto 4px 12px;
  padding: 12px;
  border-radius: 28px;
  background: var(--phone-notification);
  opacity: 0.6;
}
.push-lock-actions {
  display: flex;
  justify-content: space-between;
  padding: 0 18px 14px;
}
.push-lock-actions span {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: var(--phone-notification);
  color: var(--phone-notification-text);
}
.push-home-bar {
  align-self: center;
  width: 110px;
  height: 4px;
  border-radius: 999px;
  background: var(--phone-clock);
  opacity: 0.8;
}
.push-meta {
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
  text-align: center;
}
.drop {
  animation: push-drop 520ms cubic-bezier(0.22, 1, 0.36, 1) both;
}
.rise {
  animation: push-rise 420ms cubic-bezier(0.22, 1, 0.36, 1) both;
}
@keyframes push-drop {
  from {
    transform: translateY(-140%);
    opacity: 0;
  }
  to {
    transform: none;
    opacity: 1;
  }
}
@keyframes push-rise {
  from {
    transform: translateY(16px) scale(0.98);
    opacity: 0;
  }
  to {
    transform: none;
    opacity: 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .drop,
  .rise {
    animation: none;
  }
}
</style>
