<script setup lang="ts">
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import DecisionSummary from './ui/DecisionSummary.vue';
import AttemptHistory from './ui/AttemptHistory.vue';
import { decisionText, utcTime, type DeliveryDetail, type Execution } from '../lib/executions';
import type { Workflow } from '../lib/workflow';

// One customer's run: where they are, what each step decided, what is still scheduled and the
// messages sent, with the selected message's decision and attempts.
const props = defineProps<{
  execution: Execution;
  delivery?: DeliveryDetail;
  workflow?: Workflow;
  version: number;
  readonly?: boolean;
  admin?: boolean;
}>();
const emit = defineEmits<{
  close: [];
  inspect: [{ kind: 'event' | 'execution' | 'message'; id: string }];
  control: [string];
  delivery: [string];
}>();
const stepLabel = (id: string) => props.workflow?.nodes.find((node) => node.id === id)?.label || id;
</script>
<template>
  <section class="execution-inspector" aria-label="Execution details">
    <div class="surface-heading">
      <h3>Customer journey</h3>
      <button aria-label="Close customer journey" @click="emit('close')">Close</button>
    </div>
    <div class="toolbar">
      <button @click="emit('inspect', { kind: 'execution', id: execution.id })">
        Open activity inspector</button
      ><button
        v-if="execution.entry_event_id"
        @click="emit('inspect', { kind: 'event', id: execution.entry_event_id })"
      >
        View entry event
      </button>
    </div>
    <button
      v-if="!readonly && ['RUNNING', 'WAITING'].includes(execution.state)"
      @click="emit('control', 'cancel')"
    >
      Cancel execution</button
    ><button v-if="admin && execution.state === 'FAILED'" @click="emit('control', 'retry')">
      Recover failed job
    </button>
    <div class="execution-overview">
      <StatusBadge :status="execution.state" /><span>Version {{ version }}</span
      ><span>{{ execution.customer_id }}</span
      ><span v-if="execution.exit_reason"
        >Exit: {{ execution.exit_reason.replaceAll('_', ' ') }}</span
      >
    </div>
    <dl class="execution-facts">
      <div>
        <dt>Entered</dt>
        <dd>{{ utcTime(execution.started_at) }}</dd>
      </div>
      <div>
        <dt>Current step</dt>
        <dd>{{ stepLabel(execution.current_node_id) }}</dd>
      </div>
      <div>
        <dt>Execution / correlation ID</dt>
        <dd>{{ execution.id }}</dd>
      </div>
    </dl>
    <h4>Transitions &amp; conditions</h4>
    <div class="execution-timeline">
      <div
        v-for="step in execution.steps"
        :key="step.node_id"
        class="timeline-step"
        :class="step.state.toLowerCase()"
      >
        <span class="timeline-node"
          ><AppIcon
            :name="step.state === 'WAITING' ? 'clock' : step.state === 'FAILED' ? 'close' : 'check'"
            :size="15"
        /></span>
        <div class="timeline-step-content">
          <div>
            <strong>{{ stepLabel(step.node_id) }}</strong
            ><StatusBadge :status="step.state" />
          </div>
          <p>{{ decisionText(step.output) }}</p>
          <time
            >{{ utcTime(step.started_at)
            }}<span v-if="step.completed_at">
              · finished {{ utcTime(step.completed_at) }}</span
            ></time
          >
        </div>
      </div>
    </div>
    <h4>Wait deadlines &amp; scheduled jobs</h4>
    <p
      v-if="
        !execution.waiters.some((w) => w.state === 'WAITING') &&
        !execution.jobs.some((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))
      "
      class="muted"
    >
      No pending waits or scheduled work.
    </p>
    <div
      v-for="waiter in execution.waiters.filter((w) => w.state === 'WAITING')"
      :key="waiter.node_id"
      class="execution-wait"
    >
      <AppIcon name="events" :size="17" />
      <div>
        <strong>Waiting for {{ waiter.event_name }}</strong>
        <p>{{ stepLabel(waiter.node_id) }} · until {{ utcTime(waiter.deadline) }}</p>
      </div>
    </div>
    <div
      v-for="job in execution.jobs.filter((j) => ['SCHEDULED', 'CLAIMED'].includes(j.state))"
      :key="job.id"
      class="execution-wait"
    >
      <AppIcon name="clock" :size="17" />
      <div>
        <strong>{{ stepLabel(job.node_id) }}</strong>
        <p>{{ job.kind }} · due {{ utcTime(job.due_at) }} · {{ job.attempts }} attempts</p>
      </div>
      <StatusBadge :status="job.state" />
    </div>
    <h4>Messages &amp; decisions</h4>
    <p v-if="!execution.deliveries.length" class="muted">No messages created for this execution.</p>
    <button
      v-for="d in execution.deliveries"
      :key="d.id"
      class="list-row"
      @click="emit('delivery', d.id)"
    >
      <span
        >{{ stepLabel(d.node_id) }} · {{ d.channel.toUpperCase() }} · {{ d.status }} ·
        {{ d.attempts }} attempts</span
      ><AppIcon name="arrow" :size="15" />
    </button>
    <section v-if="delivery" class="delivery-inspector">
      <div class="surface-heading">
        <h4>Delivery decision &amp; attempts</h4>
        <button @click="emit('delivery', delivery.id)">Refresh delivery</button>
      </div>
      <DecisionSummary :decision="delivery.decision" /><AttemptHistory
        :attempts="delivery.attempt_history"
        :provider="delivery.provider"
        :response="delivery.last_error"
        :engagement="delivery.engagement"
      />
    </section>
    <details class="execution-raw">
      <summary>Advanced execution payload</summary>
      <pre>{{ JSON.stringify(execution, null, 2) }}</pre>
    </details>
  </section>
</template>
