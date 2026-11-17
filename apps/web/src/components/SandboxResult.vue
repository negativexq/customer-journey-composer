<script setup lang="ts">
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import { sandboxTime, stepDecision, type RunState } from '../lib/test-lab';

// The result of a test run or a path preview: its checks, each step at its simulated time,
// the sandbox messages and the full trace.
defineProps<{ state?: RunState; preview: boolean }>();
</script>
<template>
  <section class="surface execution-preview">
    <div class="surface-heading">
      <div>
        <h2>{{ preview ? 'Journey path preview' : 'Execution timeline' }}</h2>
        <p>
          {{
            state
              ? 'Follow each decision at its simulated time.'
              : 'Your test results will appear here.'
          }}
        </p>
      </div>
      <StatusBadge v-if="state" :status="state.result" />
    </div>
    <div v-if="!state" class="test-start-empty">
      <span class="empty-icon"><AppIcon name="flask" :size="32" /></span>
      <h3>Ready for a test drive?</h3>
      <p>Choose a scenario, create a test run, then use the virtual clock to explore its path.</p>
      <div>
        <span><AppIcon name="shield" :size="13" />No live profile changes</span
        ><span><AppIcon name="email" :size="13" />No provider calls</span>
      </div>
    </div>
    <template v-else
      ><div class="run-result-heading">
        <span
          >Result <strong data-testid="test-result">{{ state.result }}</strong></span
        ><span>{{ state.status }} {{ state.paused ? '· paused' : '' }}</span
        ><span v-if="state.exit_reason">{{ state.exit_reason.replaceAll('_', ' ') }}</span>
      </div>
      <div class="assertion-list">
        <span v-for="(a, i) in state.assertions" :key="i" :class="{ passed: a.pass }"
          ><AppIcon :name="a.pass ? 'check' : 'close'" :size="13" />{{
            a.description.replaceAll('_', ' ')
          }}
          <small>{{ a.actual }}</small></span
        >
      </div>
      <div class="execution-timeline">
        <div
          v-for="(s, i) in state.steps"
          :key="i"
          class="timeline-step"
          :class="s.state.toLowerCase()"
        >
          <span class="timeline-node"
            ><AppIcon
              :name="s.state === 'WAITING' ? 'clock' : s.state === 'FAILED' ? 'close' : 'check'"
              :size="15"
          /></span>
          <div class="timeline-step-content">
            <div>
              <strong>{{ s.label }}</strong
              ><StatusBadge :status="s.state" />
            </div>
            <p>{{ stepDecision(s.output) }}</p>
            <time
              >{{ sandboxTime(s.started_at)
              }}<span v-if="s.deadline"> · deadline {{ sandboxTime(s.deadline) }}</span></time
            >
          </div>
        </div>
      </div>
      <div class="sandbox-output">
        <h3>
          Sandbox messages <span class="count-label">{{ state.messages.length }}</span>
        </h3>
        <p v-if="!state.messages.length" class="muted">No messages produced.</p>
        <div v-for="m in state.messages" :key="m.id" class="sandbox-message-row">
          <span class="channel-icon" :class="m.channel"
            ><AppIcon :name="m.channel === 'email' ? 'email' : 'message'" :size="16"
          /></span>
          <div>
            <strong>{{ m.channel === 'email' ? 'Email' : 'SMS' }} message</strong
            ><small>{{ sandboxTime(m.scheduled_at) }}</small>
          </div>
          <StatusBadge :status="m.status" />
        </div>
        <p class="card-footnote">
          Open Test Inbox to inspect rendered content and delivery decisions.
        </p>
      </div>
      <details class="trace-details">
        <summary>
          <AppIcon name="file" :size="14" />Execution trace ({{ state.trace.length }})
        </summary>
        <p v-for="(t, i) in state.trace" :key="i">
          {{ sandboxTime(t.at) }} · {{ t.kind }} · {{ t.message }}
        </p>
      </details></template
    >
  </section>
</template>
