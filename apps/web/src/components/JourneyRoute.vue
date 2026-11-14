<script setup lang="ts">
import AppIcon from './ui/AppIcon.vue';
import { formatDateTime, plural } from '../lib/format';
import type { JourneyPath } from '../lib/customer-journey';
import { labels } from '../lib/workflow';

// The stops one person made, in order, with what happened at each. With `focusable`, a stop is a
// button that asks the owner to show it on a map.
defineProps<{ path: JourneyPath; focused?: string; focusable?: boolean }>();
const emit = defineEmits<{ focus: [string]; message: [string] }>();
const icons: Record<string, string> = {
  event_trigger: 'zap',
  wait: 'clock',
  wait_event: 'events',
  condition: 'branch',
  email: 'email',
  sms: 'message',
  update_attribute: 'user',
  exit: 'exit',
};
</script>
<template>
  <div class="route">
    <h4>
      Route taken <span>{{ plural(path.steps.length, 'step') }}</span>
    </h4>
    <ol>
      <li
        v-for="step in path.steps"
        :key="step.key"
        class="route-step"
        :class="[step.tone, { focused: focused === step.nodeID }]"
        :data-node="step.nodeID"
      >
        <span class="route-rail" aria-hidden="true"
          ><span class="step-type-icon" :class="step.kind"
            ><AppIcon :name="icons[step.kind] || 'workflow'" :size="16" /></span
        ></span>
        <div class="route-copy">
          <component
            :is="focusable ? 'button' : 'div'"
            :type="focusable ? 'button' : undefined"
            class="route-focus"
            :aria-label="focusable ? `Show ${step.label} on the map` : undefined"
            @click="focusable && emit('focus', step.nodeID)"
          >
            <span class="route-label">{{ step.label }}</span>
            <span v-if="step.branch" class="route-branch">{{ step.branch }}</span>
            <span v-if="step.tone === 'current'" class="route-now">Now</span>
          </component>
          <p class="route-outcome">{{ step.outcome }}</p>
          <p class="route-detail">
            {{ labels[step.kind] }}<template v-if="step.detail"> · {{ step.detail }}</template>
          </p>
          <p class="route-time">
            {{ formatDateTime(step.at, { seconds: true })
            }}<template v-if="step.took"> · {{ step.took }} here</template>
            <button
              v-if="step.deliveryID"
              type="button"
              class="route-link"
              @click="emit('message', step.deliveryID)"
            >
              View message
            </button>
          </p>
        </div>
      </li>
    </ol>
    <div v-if="path.notReached.length" class="not-reached">
      <p>{{ plural(path.notReached.length, 'step') }} not on this route</p>
      <div>
        <component
          :is="focusable ? 'button' : 'span'"
          v-for="step in path.notReached"
          :key="step.id"
          :type="focusable ? 'button' : undefined"
          :aria-label="focusable ? `Show ${step.label} on the map` : undefined"
          @click="focusable && emit('focus', step.id)"
        >
          {{ step.label }}
        </component>
      </div>
    </div>
  </div>
</template>
<style scoped>
.route h4 {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 0 0 12px;
  font-size: 13px;
}
.route h4 span {
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
}
ol {
  list-style: none;
  margin: 0;
  padding: 0;
}
.route-step {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 12px;
  border-radius: 8px;
}
.route-rail {
  position: relative;
  display: flex;
  justify-content: center;
}
/* The line that joins one stop to the next. */
.route-step:not(:last-child) .route-rail::after {
  content: '';
  position: absolute;
  top: 32px;
  bottom: -2px;
  width: 2px;
  border-radius: 1px;
  background: var(--accent);
  opacity: 0.35;
}
.route-copy {
  padding-bottom: 18px;
  min-width: 0;
}
.route-focus {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
}
.route-focus:hover {
  background: none;
}
.route-focus:hover .route-label,
.focused .route-label {
  color: var(--accent-text);
}
.route-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}
.route-branch,
.route-now {
  padding: 2px 6px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 600;
  background: var(--accent-soft);
  color: var(--accent-text);
}
.route-now {
  background: var(--amber-97);
  color: var(--amber-51-text);
}
.route-outcome {
  margin: 2px 0 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--text);
}
.blocked .route-outcome {
  color: var(--amber-51-text);
}
.failed .route-outcome {
  color: var(--red-49-text);
}
.route-detail,
.route-time {
  margin: 3px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.route-link {
  min-height: 0;
  margin-left: 6px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--accent-text);
  font-size: 12px;
  text-decoration: underline;
}
.route-link:hover {
  background: none;
}
.current .step-type-icon {
  box-shadow: 0 0 0 3px var(--amber-90);
}
.not-reached {
  padding: 12px 0;
  border-top: 1px dashed var(--border);
}
.not-reached p {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--muted);
}
.not-reached div {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.not-reached button,
.not-reached span {
  display: inline-flex;
  align-items: center;
  border: 1px dashed var(--gray-91);
  border-radius: 7px;
  background: none;
  min-height: 26px;
  padding: 3px 8px;
  border-style: dashed;
  color: var(--muted);
  font-size: 12px;
}
div.route-focus {
  cursor: default;
}
</style>
