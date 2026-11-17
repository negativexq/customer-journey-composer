<script setup lang="ts">
import { computed } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import { formatDateTime, plural } from '../lib/format';
import { elapsed, readableTrace, type TraceEntry, type TraceItem } from '../lib/trace';

const props = defineProps<{ trace: unknown[]; sourceNode?: string }>();
const entries = computed(() => (Array.isArray(props.trace) ? (props.trace as TraceEntry[]) : []));
const items = computed(() => readableTrace(entries.value, (iso) => formatDateTime(iso)));
const start = computed(() => entries.value[0]?.at || '');
const multiple = computed(
  () => new Set(entries.value.map((e) => e.execution).filter((n) => n > 0)).size > 1,
);
const steps = computed(() => items.value.filter((i) => i.type === 'step').length);
const span = computed(() => {
  const last = entries.value.at(-1)?.at;
  return last && start.value ? elapsed(start.value, last) : 'start';
});
const ending = computed(() => {
  const exit = [...entries.value].reverse().find((e) => e.kind === 'exit');
  return exit ? exit.message.replace(/^Global exit: /, '').replaceAll('_', ' ') : 'still running';
});

function iconFor(item: TraceItem): string {
  if (item.type === 'note')
    return item.kind === 'event'
      ? 'zap'
      : item.kind === 'attributes'
        ? 'user'
        : item.kind === 'error'
          ? 'close'
          : 'history';
  const kinds = item.details.map((d) => d.text);
  if (kinds.some((t) => t.startsWith('Left the journey'))) return 'exit';
  if (kinds.some((t) => t.startsWith('Message produced'))) return 'message';
  if (kinds.some((t) => t.startsWith('Waits until'))) return 'clock';
  if (kinds.some((t) => t.includes('path'))) return 'split';
  if (kinds.some((t) => t.startsWith('Would call'))) return 'webhook';
  if (kinds.some((t) => t === 'Customer enrolled')) return 'play';
  return 'workflow';
}
// A time label only where the virtual clock moved, so the rail reads like a schedule.
function showTime(index: number) {
  return index === 0 || items.value[index - 1].at !== items.value[index].at;
}
function newExecution(index: number) {
  if (!multiple.value) return false;
  const current = items.value[index].execution;
  return current > 0 && (index === 0 || items.value[index - 1].execution !== current);
}
</script>
<template>
  <div class="trace">
    <p v-if="!entries.length" class="trace-empty">
      This message has no recorded trace. Traces come from Test Lab runs.
    </p>
    <template v-else>
      <dl class="trace-summary">
        <div>
          <dt>Steps</dt>
          <dd>{{ plural(steps, 'step') }}</dd>
        </div>
        <div>
          <dt>Simulated time</dt>
          <dd>{{ span === 'start' ? 'Instant' : span.slice(1) }}</dd>
        </div>
        <div>
          <dt>Outcome</dt>
          <dd>{{ ending }}</dd>
        </div>
      </dl>
      <ol class="trace-list">
        <template v-for="(item, i) in items" :key="i">
          <li v-if="newExecution(i)" class="trace-execution">Execution {{ item.execution }}</li>
          <li
            class="trace-item"
            :class="[
              item.type,
              item.type === 'note' ? 'trace-tone-' + item.tone : '',
              { current: item.type === 'step' && item.nodeID === sourceNode },
            ]"
          >
            <time
              class="trace-time"
              :datetime="item.at"
              :title="formatDateTime(item.at, { seconds: true })"
              >{{ showTime(i) ? elapsed(start, item.at) : '' }}</time
            >
            <span class="trace-dot" aria-hidden="true"
              ><AppIcon :name="iconFor(item)" :size="13"
            /></span>
            <div class="trace-body">
              <template v-if="item.type === 'step'">
                <div class="trace-head">
                  <strong>{{ item.label }}</strong>
                  <span v-if="item.nodeID === sourceNode" class="trace-this">This message</span>
                </div>
                <ul v-if="item.details.length" class="trace-details">
                  <li
                    v-for="(d, j) in item.details"
                    :key="j"
                    :class="'trace-tone-' + (d.tone || 'neutral')"
                  >
                    <span>{{ d.text }}</span>
                    <span
                      v-if="d.status"
                      class="trace-pill"
                      :class="'trace-tone-' + (d.tone || 'neutral')"
                      >{{ d.status }}</span
                    >
                  </li>
                </ul>
              </template>
              <div v-else class="trace-head">
                <span v-if="item.caption" class="trace-caption">{{ item.caption }}</span>
                <strong>{{ item.subject || item.title }}</strong>
                <span v-if="item.status" class="trace-pill" :class="'trace-tone-' + item.tone">{{
                  item.status
                }}</span>
              </div>
            </div>
          </li>
        </template>
      </ol>
      <details class="trace-raw">
        <summary>Raw trace (JSON)</summary>
        <pre>{{ JSON.stringify(entries, null, 2) }}</pre>
      </details>
    </template>
  </div>
</template>
<style scoped>
.trace {
  display: grid;
  gap: 20px;
  padding-top: 4px;
}
.trace-empty {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
}
.trace-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}
.trace-summary div {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.trace-summary dt {
  font-size: 11px;
  color: var(--muted);
}
.trace-summary dd {
  margin: 3px 0 0;
  font-size: 13px;
  font-weight: 600;
}
.trace-summary dd::first-letter {
  text-transform: uppercase;
}
.trace-list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.trace-item {
  position: relative;
  display: grid;
  grid-template-columns: 52px 26px minmax(0, 1fr);
  gap: 10px;
  padding-bottom: 14px;
}
/* The rail joins one step to the next. */
.trace-item::before {
  content: '';
  position: absolute;
  top: 26px;
  bottom: 0;
  left: 74px;
  width: 1px;
  background: var(--border);
}
.trace-item:last-child::before {
  display: none;
}
.trace-time {
  padding-top: 5px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-align: right;
  color: var(--muted);
  white-space: nowrap;
}
.trace-dot {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border: 1px solid var(--border);
  border-radius: 50%;
  color: var(--muted);
  background: var(--surface);
}
.trace-item.step .trace-dot {
  color: var(--accent-text);
  border-color: var(--accent-soft);
  background: var(--accent-soft);
}
.trace-item.current .trace-dot {
  color: var(--on-solid);
  border-color: var(--accent);
  background: var(--accent);
}
.trace-item.trace-tone-green .trace-dot {
  color: var(--green-47-text);
}
.trace-item.trace-tone-amber .trace-dot {
  color: var(--amber-55-text);
}
.trace-item.trace-tone-red .trace-dot {
  color: var(--red-56);
}
.trace-body {
  min-width: 0;
  padding-top: 3px;
}
.trace-item.step .trace-body {
  padding: 7px 10px;
  margin-top: -3px;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.trace-item.current .trace-body {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}
.trace-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
  font-size: 13px;
}
.trace-item.note .trace-body {
  padding: 8px 10px;
  margin-top: -3px;
  border: 1px dashed var(--border);
  border-radius: 8px;
}
.trace-item.note.trace-tone-red .trace-body {
  border-style: solid;
  border-color: var(--red-56);
}
.trace-caption {
  flex-basis: 100%;
  font-size: 11px;
  color: var(--muted);
}
.trace-item.note strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
}
.trace-this {
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  background: var(--accent-soft);
}
.trace-details {
  display: grid;
  gap: 3px;
  margin: 5px 0 0;
  padding: 0;
  list-style: none;
}
.trace-details li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}
.trace-details li::before {
  content: '';
  flex: none;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.6;
}
.trace-details li.trace-tone-green::before {
  background: var(--green-54);
  opacity: 1;
}
.trace-details li.trace-tone-blue::before {
  background: var(--blue-69);
  opacity: 1;
}
.trace-details li.trace-tone-amber::before {
  background: var(--amber-62);
  opacity: 1;
}
.trace-details li.trace-tone-red {
  color: var(--red-56);
}
.trace-pill {
  padding: 0 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
  color: var(--muted);
  background: var(--gray-96);
}
.trace-pill.trace-tone-green {
  color: var(--green-43-text);
  background: var(--green-94);
}
.trace-pill.trace-tone-blue {
  color: var(--blue-56-text);
  background: var(--blue-96);
}
.trace-pill.trace-tone-amber {
  color: var(--amber-53-text);
  background: var(--amber-94);
}
.trace-pill.trace-tone-red {
  color: var(--red-64-text);
  background: var(--red-96);
}
.trace-execution {
  margin: 4px 0 10px 88px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
}
.trace-raw summary {
  cursor: pointer;
  font-size: 12px;
  color: var(--muted);
}
.trace-raw pre {
  margin-top: 8px;
}
@media (max-width: 760px) {
  .trace-item {
    grid-template-columns: 40px 26px minmax(0, 1fr);
    gap: 8px;
  }
  .trace-item::before {
    left: 60px;
  }
  .trace-execution {
    margin-left: 74px;
  }
}
</style>
