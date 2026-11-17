<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import SegmentedControl from './ui/SegmentedControl.vue';
import { formatDateTime, plural } from '../lib/format';
import type { TestMessage } from '../lib/test-inbox';

// A test message as the customer would see it: an email in a mail window or on a phone, or a
// text message in a phone thread.
const props = defineProps<{ detail: TestMessage }>();
// Desktop or phone view of an email; the inbox keeps the choice from message to message.
const device = defineModel<string>('device', { required: true });
// Desktop email renders at a typical client width and is scaled down to fit the column.
const MAIL_WIDTH = 640;
const MAIL_HEIGHT = 760;
const mailBody = ref<HTMLElement>();
const mailDialog = ref<HTMLDialogElement>();
const mailOpen = ref(false);
function openMail() {
  mailOpen.value = true;
  mailDialog.value?.showModal();
}
const mailScale = ref(1);
const mailHeight = computed(() => Math.round(MAIL_HEIGHT * mailScale.value));
const mailObserver =
  typeof ResizeObserver === 'undefined'
    ? undefined
    : new ResizeObserver(([entry]) => {
        mailScale.value = Math.min(1, entry.contentRect.width / MAIL_WIDTH);
      });
watch(mailBody, (element, old) => {
  if (old) mailObserver?.unobserve(old);
  if (element) mailObserver?.observe(element);
});
// The phone shows the email at a real phone's width (390 px) scaled to its screen.
const PHONE_WIDTH = 390;
const phoneScreen = ref<HTMLElement>();
const phoneScale = ref(0.77);
const phoneObserver =
  typeof ResizeObserver === 'undefined'
    ? undefined
    : new ResizeObserver(([entry]) => {
        if (entry.contentRect.width) phoneScale.value = entry.contentRect.width / PHONE_WIDTH;
      });
watch(phoneScreen, (element, old) => {
  if (old) phoneObserver?.unobserve(old);
  if (element) phoneObserver?.observe(element);
});
onBeforeUnmount(() => {
  mailObserver?.disconnect();
  phoneObserver?.disconnect();
});
const escape = (s: string) =>
  s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const document = computed(
  () =>
    `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none';style-src 'unsafe-inline';img-src data:;form-action 'none';base-uri 'none'"><meta name="viewport" content="width=device-width"><style>body{margin:0;font:16px/1.6 Arial}body>:not(table){margin-left:20px;margin-right:20px}img{max-width:100%}</style></head><body>${props.detail.message.html || '<pre>' + escape(props.detail.message.text || '') + '</pre>'}</body></html>`,
);
// SMS text split so links can be drawn as a phone draws them.
const smsParts = computed(() =>
  (props.detail.message.text || '')
    .split(/(https?:\/\/\S+)/)
    .filter(Boolean)
    .map((text) => ({ text, link: /^https?:\/\//.test(text) })),
);
</script>
<template>
  <SegmentedControl
    v-if="detail.channel === 'email'"
    v-model="device"
    class="device-segments"
    label="Device"
    hide-label
    :options="[
      { value: 'desktop', label: 'Desktop', icon: 'monitor' },
      { value: 'mobile', label: 'Mobile' },
    ]"
  /><!-- A mail client window, so the desktop view reads as an opened email. -->
  <div v-if="detail.channel === 'email' && device === 'desktop'" class="mail-stage">
    <div class="mail-window">
      <div class="mail-titlebar">
        <span aria-hidden="true"><i /><i /><i /></span>
        <button type="button" class="mail-expand" @click="openMail">
          <AppIcon name="maximize" :size="13" />Open full size
        </button>
      </div>
      <header class="mail-header">
        <span class="mail-avatar" aria-hidden="true"><AppIcon name="email" :size="16" /></span>
        <div>
          <strong>{{ detail.message.subject || 'No subject' }}</strong>
          <span
            >To {{ detail.message.to || detail.customer_id }} ·
            {{ formatDateTime(detail.simulated_at) }}</span
          >
          <small v-if="detail.message.preheader">{{ detail.message.preheader }}</small>
        </div>
      </header>
      <div ref="mailBody" class="mail-body" :style="{ height: mailHeight + 'px' }">
        <iframe
          title="Test email preview"
          sandbox=""
          :srcdoc="document"
          :style="{ transform: `scale(${mailScale})` }"
        />
      </div>
    </div>
    <dialog
      ref="mailDialog"
      class="mail-dialog"
      aria-labelledby="mail-dialog-title"
      @close="mailOpen = false"
      @click="$event.target === mailDialog && mailDialog?.close()"
    >
      <header>
        <h2 id="mail-dialog-title">
          {{ detail.message.subject || 'Test email' }}
        </h2>
        <button
          type="button"
          class="icon-button"
          aria-label="Close full size preview"
          @click="mailDialog?.close()"
        >
          <AppIcon name="close" :size="16" />
        </button>
      </header>
      <iframe v-if="mailOpen" title="Test email preview, full size" sandbox="" :srcdoc="document" />
    </dialog>
  </div>
  <!-- The preview shows what the customer sees; technical details are under Payload. -->
  <div v-else class="inbox-phone-preview">
    <div class="phone" :class="detail.channel === 'email' ? 'phone-email' : 'phone-sms'">
      <div class="phone-status" aria-hidden="true">
        <span>9:41</span><span class="phone-island" /><span class="phone-signal"
          ><i /><i /><i /><i
        /></span>
      </div>
      <div v-if="detail.channel === 'email'" ref="phoneScreen" class="phone-screen">
        <iframe
          title="Test email preview"
          sandbox=""
          :srcdoc="document"
          :style="{
            transform: `scale(${phoneScale})`,
            height: `calc(100% / ${phoneScale})`,
          }"
        />
      </div>
      <template v-else>
        <div class="phone-thread-head">
          <span class="phone-avatar" aria-hidden="true"><AppIcon name="message" :size="16" /></span>
          <strong>{{ detail.journey_name || 'Test message' }}</strong>
          <small>Text message</small>
        </div>
        <div class="phone-thread">
          <small class="phone-time">{{ formatDateTime(detail.simulated_at) }}</small>
          <p class="sms-phone">
            <template v-for="(part, i) in smsParts" :key="i"
              ><span v-if="part.link" class="sms-link">{{ part.text }}</span
              ><template v-else>{{ part.text }}</template></template
            >
          </p>
        </div>
        <div class="phone-compose" aria-hidden="true"><span>Text message</span></div>
      </template>
      <span class="phone-home" aria-hidden="true" />
    </div>
    <small v-if="detail.channel === 'sms'" class="phone-meta"
      >{{ (detail.message.text || '').length }} characters · {{ detail.message.encoding }} ·
      {{ plural(detail.message.segments ?? 0, 'segment') }}</small
    >
  </div>
</template>
<style scoped>
.mail-stage {
  padding: 16px 12px;
  border-radius: 12px;
  background: var(--gray-96);
}
.mail-window {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  box-shadow:
    0 1px 2px #10182814,
    0 12px 32px -12px #10182833;
}
.mail-titlebar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--gray-99);
}
.mail-titlebar > span {
  display: flex;
  gap: 6px;
}
.mail-expand {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 26px;
  margin: -4px -4px -4px auto;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--muted);
  font-size: 12px;
}
.mail-expand:hover {
  background: var(--gray-96);
  color: var(--text);
}
.mail-dialog {
  width: min(760px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface);
  color: var(--text);
}
.mail-dialog::backdrop {
  background: #0f111599;
}
.mail-dialog header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 12px 12px 20px;
  border-bottom: 1px solid var(--border);
}
.mail-dialog h2 {
  flex: 1;
  margin: 0;
  font-size: 15px;
}
.mail-dialog iframe {
  display: block;
  width: 100%;
  height: min(78vh, 900px);
  border: 0;
  background: var(--phone-screen);
}
.mail-titlebar i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--edge-muted);
}
.mail-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
}
.mail-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
}
.mail-header div {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.mail-header strong {
  font-size: 14px;
  color: var(--text);
  overflow-wrap: anywhere;
}
.mail-header span,
.mail-header small {
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.mail-body {
  position: relative;
  overflow: hidden;
  background: var(--phone-screen);
}
.mail-body iframe {
  position: absolute;
  top: 0;
  left: 0;
  width: 640px;
  height: 760px;
  border: 0;
  transform-origin: 0 0;
}
.inbox-phone-preview {
  display: grid;
  justify-items: center;
  gap: 12px;
  padding: 24px 12px;
  border-radius: 12px;
  background: var(--gray-96);
}
.phone {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(100%, 320px);
  height: 600px;
  padding: 10px 12px 14px;
  border: 9px solid var(--gray-25);
  border-radius: 46px;
  background: var(--phone-screen);
  box-shadow:
    0 0 0 1px var(--gray-74),
    0 24px 48px -16px #17213b4d;
  overflow: hidden;
}
.phone-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 14px 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}
.phone-island {
  width: 86px;
  height: 24px;
  border-radius: 999px;
  background: var(--gray-25);
}
.phone-signal {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 11px;
}
.phone-signal i {
  width: 3px;
  border-radius: 1px;
  background: var(--text);
}
.phone-signal i:nth-child(1) {
  height: 4px;
}
.phone-signal i:nth-child(2) {
  height: 6px;
}
.phone-signal i:nth-child(3) {
  height: 8px;
}
.phone-signal i:nth-child(4) {
  height: 11px;
}
.phone-thread-head {
  display: grid;
  justify-items: center;
  gap: 3px;
  padding: 6px 0 12px;
  border-bottom: 1px solid var(--border);
}
.phone-avatar {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: linear-gradient(160deg, var(--gray-74), var(--gray-55));
  color: var(--on-solid);
}
.phone-thread-head strong {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--text);
}
.phone-thread-head small {
  font-size: 11px;
  color: var(--muted);
}
.phone-thread {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 14px 4px;
  overflow: auto;
}
.phone-time {
  align-self: center;
  font-size: 11px;
  color: var(--muted);
}
.sms-phone {
  max-width: 82%;
  margin: 0;
  padding: 9px 13px;
  border-radius: 18px 18px 18px 5px;
  background: var(--phone-bubble);
  color: var(--text);
  font-size: 14px;
  line-height: 1.4;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.sms-link {
  color: var(--phone-link);
  text-decoration: underline;
}
.phone-compose {
  display: flex;
  align-items: center;
  min-height: 34px;
  margin: 0 4px 10px;
  padding: 0 14px;
  border: 1px solid var(--border);
  border-radius: 999px;
  font-size: 13px;
  color: var(--gray-60);
}
.phone-home {
  align-self: center;
  width: 110px;
  height: 4px;
  border-radius: 999px;
  background: var(--text);
  opacity: 0.8;
}
.phone-screen {
  position: relative;
  flex: 1;
  margin: 0 -12px 8px;
  overflow: hidden;
}
.phone-screen iframe {
  position: absolute;
  top: 0;
  left: 0;
  width: 390px;
  border: 0;
  border-radius: 0;
  transform-origin: 0 0;
}
.phone-meta {
  font-size: 12px;
  color: var(--muted);
}
</style>
