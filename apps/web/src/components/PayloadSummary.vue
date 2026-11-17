<script setup lang="ts">
import { computed } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import type { TestMessage } from '../lib/test-inbox';

// What went into a test message: its template and step, the IDs that trace it, the values it
// was personalized with and the raw record.
const props = defineProps<{ detail: TestMessage }>();
const emit = defineEmits<{ copy: [value: string, notice: string] }>();
const personalization = computed(() =>
  Object.entries(props.detail.message.personalization_values || {}),
);
const payloadIds = computed(() => [
  { label: 'Trace', icon: 'history', value: props.detail.trace_id, empty: 'Not recorded' },
  {
    label: 'Workflow snapshot',
    icon: 'lock',
    value: props.detail.workflow_hash,
    empty: 'Standalone message',
  },
]);
// Long IDs stay readable at a glance; the full value is in the tooltip and copy button.
const shortId = (value: string) => (value.length > 16 ? value.slice(0, 12) + '…' : value);
</script>
<template>
  <dl class="payload-summary">
    <div>
      <dt><AppIcon name="templates" :size="14" />Template</dt>
      <dd>{{ detail.message.template_id || 'Inline content' }}</dd>
    </div>
    <div>
      <dt><AppIcon name="workflow" :size="14" />Source step</dt>
      <dd>{{ detail.source_node || 'Explicit test' }}</dd>
    </div>
    <div v-for="id in payloadIds" :key="id.label" class="payload-id">
      <dt><AppIcon :name="id.icon" :size="14" />{{ id.label }}</dt>
      <dd>
        <code :title="id.value">{{ id.value ? shortId(id.value) : id.empty }}</code>
        <button
          v-if="id.value"
          type="button"
          class="icon-button payload-copy"
          :aria-label="`Copy ${id.label.toLowerCase()} ID`"
          @click="emit('copy', id.value, `${id.label} ID copied`)"
        >
          <AppIcon name="copy" :size="13" />
        </button>
      </dd>
    </div>
  </dl>
  <section class="payload-section" aria-labelledby="personalization-heading">
    <h3 id="personalization-heading">
      Personalization values
      <span v-if="personalization.length" class="payload-count">{{ personalization.length }}</span>
    </h3>
    <ul v-if="personalization.length" class="token-list">
      <li v-for="[key, value] in personalization" :key="key">
        <code class="token-key">{{ key }}</code>
        <AppIcon name="arrow" :size="14" class="token-arrow" />
        <span class="token-value">{{ value || 'Empty' }}</span>
      </li>
    </ul>
    <p v-else class="help-text">This message uses no personalization.</p>
  </section>
  <details class="raw-payload">
    <summary>
      <AppIcon name="file" :size="14" />Raw payload
      <span>JSON</span>
      <AppIcon name="down" :size="14" class="raw-chevron" />
    </summary>
    <pre>{{ JSON.stringify(detail, null, 2) }}</pre>
  </details>
</template>
<style scoped>
/* The raw record scrolls inside the panel instead of stretching it. */
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 650px;
  overflow: auto;
}
.payload-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 0 0 20px;
}
.payload-summary > div {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 11px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--gray-99);
}
.payload-summary dt {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}
.payload-summary dt :deep(svg) {
  color: var(--accent-text);
}
.payload-summary dd {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  overflow-wrap: anywhere;
}
.payload-id code {
  min-width: 0;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.payload-copy {
  flex: none;
  width: 28px;
  height: 28px;
  margin: -4px -4px -4px auto;
}
.payload-section h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 14px;
}
.payload-count {
  min-width: 20px;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-size: 12px;
  text-align: center;
}
.token-list {
  display: grid;
  gap: 8px;
  margin: 0 0 20px;
  padding: 0;
  list-style: none;
}
.token-list li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
}
.token-key {
  justify-self: start;
  max-width: 100%;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--accent-soft);
  color: var(--accent-text-strong);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.token-arrow {
  color: var(--muted);
}
.token-value {
  margin-left: auto;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  overflow-wrap: anywhere;
}
.raw-payload {
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}
.raw-payload summary {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  cursor: pointer;
  list-style: none;
}
.raw-payload summary::-webkit-details-marker {
  display: none;
}
.raw-payload summary:hover {
  background: var(--gray-99);
}
.raw-payload summary span {
  padding: 1px 6px;
  border-radius: 5px;
  background: var(--gray-96);
  color: var(--muted);
  font-size: 11px;
  font-weight: 500;
}
.raw-chevron {
  margin-left: auto;
  color: var(--muted);
  transition: transform 0.2s ease;
}
.raw-payload[open] .raw-chevron {
  transform: rotate(180deg);
}
.raw-payload pre {
  margin: 0;
  border-top: 1px solid var(--border);
  border-radius: 0;
}
@media (max-width: 520px) {
  .payload-summary {
    grid-template-columns: 1fr;
  }
}
</style>
