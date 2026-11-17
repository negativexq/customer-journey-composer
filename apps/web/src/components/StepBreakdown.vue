<script lang="ts">
export type JourneyMessage = {
  id: string;
  execution_id: string;
  customer_id: string;
  node_id: string;
  channel: string;
  status: string;
  decision: unknown;
  attempts: number;
  created_at: string;
};
</script>
<script setup lang="ts">
import { computed } from 'vue';
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'reka-ui';
import SkeletonBlock from './ui/SkeletonBlock.vue';
import ScopeSelect from './ui/ScopeSelect.vue';
import { portLabel } from '../lib/workflow-presentation';
import { metric, reasonLabel, type Insights } from '../lib/insights';
import { isMessage, type Workflow } from '../lib/workflow';

// The counts, paths and messages of one step, or the journey's messages when no step is
// chosen. Every list it opens is loaded by the monitor, which owns the filters.
const props = defineProps<{
  step?: Workflow['nodes'][number];
  nodeId: string;
  // The branch the executions list is narrowed to, if any.
  branch: string;
  showMessages: boolean;
  workflow: Workflow;
  insights: Insights;
  messages: JourneyMessage[];
  busy: boolean;
  more: boolean;
  closable: boolean;
}>();
const emit = defineEmits<{
  close: [];
  branch: [string];
  reason: [string];
  exits: [string];
  waiting: [];
  load: [boolean];
  'open-message': [JourneyMessage];
}>();
const tab = defineModel<string>('tab', { required: true });
const channel = defineModel<string>('channel', { required: true });
const status = defineModel<string>('status', { required: true });
const reason = defineModel<string>('reason', { required: true });
const value = (dimension: string, name: string, key = '') =>
  metric(props.insights, dimension, name, key);
const nodeValue = (name: string, dimension = 'node') => value(dimension, name, props.nodeId);
const nodeReasons = computed(() =>
  props.insights.metrics.filter(
    (m) => m.dimension === 'node_reason' && m.key === props.nodeId && m.value > 0,
  ),
);
const stepLabel = (id: string) => props.workflow.nodes.find((node) => node.id === id)?.label || id;
</script>
<template>
  <section :aria-label="step ? 'Step breakdown' : 'Message breakdown'">
    <div class="surface-heading">
      <div>
        <small>{{ step ? 'STEP DETAILS' : 'MESSAGE DETAILS' }}</small>
        <h3>{{ step?.label || 'Journey messages' }}</h3>
      </div>
      <button v-if="closable" aria-label="Close step breakdown" @click="emit('close')">
        Close
      </button>
    </div>
    <TabsRoot v-model="tab"
      ><TabsList class="observe-tabs" aria-label="Step detail tabs"
        ><TabsTrigger v-if="step" value="summary">Summary</TabsTrigger
        ><TabsTrigger v-if="step" value="people">People</TabsTrigger
        ><TabsTrigger v-if="showMessages || isMessage(step?.type)" value="messages"
          >Messages</TabsTrigger
        ></TabsList
      ><TabsContent value="summary"
        ><dl class="node-detail-stats">
          <div>
            <dt>Reached step</dt>
            <dd>{{ nodeValue('entered') }}</dd>
          </div>
          <div>
            <dt>Waiting now</dt>
            <dd>{{ nodeValue('WAITING', 'node_current') }}</dd>
          </div>
          <div>
            <dt>Step completed</dt>
            <dd>{{ nodeValue('SUCCEEDED') }}</dd>
          </div>
          <div>
            <dt>
              {{ isMessage(step?.type) ? 'Provider accepted' : 'Processing now' }}
            </dt>
            <dd>
              {{
                isMessage(step?.type)
                  ? nodeValue('ACCEPTED', 'node_delivery')
                  : nodeValue('RUNNING', 'node_current')
              }}
            </dd>
          </div>
        </dl>
        <dl v-if="isMessage(step?.type)" class="node-detail-stats" aria-label="Engagement">
          <div v-if="step?.type === 'email'">
            <dt>Opened</dt>
            <dd>{{ nodeValue('opened', 'node_delivery') }}</dd>
          </div>
          <div>
            <dt>{{ step?.type === 'push' ? 'Tapped' : 'Clicked a link' }}</dt>
            <dd>{{ nodeValue('clicked', 'node_delivery') }}</dd>
          </div>
          <div>
            <dt>{{ step?.type === 'push' ? 'Device token expired' : 'Bounced' }}</dt>
            <dd>{{ nodeValue('bounced', 'node_delivery') }}</dd>
          </div>
          <div v-if="step?.type === 'email'">
            <dt>Unsubscribed</dt>
            <dd>{{ nodeValue('unsubscribed', 'node_delivery') }}</dd>
          </div>
        </dl>
        <p v-if="isMessage(step?.type)" class="help-text">
          Completing a message step schedules the message; provider acceptance is a separate result.
          Each person counts once however often they open or click. Opens count when the mail client
          loads images, which some clients do on their own.
        </p>
        <h4>Paths taken</h4>
        <button
          v-for="edge in workflow.edges.filter((e) => e.source === nodeId)"
          :key="edge.id"
          class="list-row"
          @click="emit('branch', edge.port)"
        >
          <span>{{ portLabel(edge.port) }} → {{ stepLabel(edge.target) }}</span
          ><strong>{{ value('edge', 'traversed', nodeId + ':' + edge.port) }}</strong>
        </button>
        <h4>Message decisions</h4>
        <p v-if="!nodeReasons.length" class="help-text">
          No blocked or failed decisions in this scope.
        </p>
        <button
          v-for="r in nodeReasons"
          :key="r.metric"
          class="list-row"
          @click="emit('reason', r.metric)"
        >
          <span>{{ reasonLabel(r.metric) }}</span
          ><strong>{{ r.value }}</strong>
        </button>
        <div
          v-if="
            insights.metrics.some(
              (m) => m.dimension === 'node_exit' && m.key === nodeId && m.value > 0,
            )
          "
        >
          <h4>Global exits from this step</h4>
          <button
            v-for="r in insights.metrics.filter(
              (m) => m.dimension === 'node_exit' && m.key === nodeId && m.value > 0,
            )"
            :key="r.metric"
            class="list-row"
            @click="emit('exits', r.metric)"
          >
            <span>{{ reasonLabel(r.metric) }}</span
            ><strong>{{ r.value }}</strong>
          </button>
          <p class="help-text">
            These customers left directly from this step, without visiting an Exit node.
          </p>
        </div></TabsContent
      >
      <TabsContent value="people"
        ><p>
          The execution list shows the people behind this step{{
            branch ? ' and selected branch' : ''
          }}. Select a person to inspect their path and timeline.
        </p>
        <button @click="emit('waiting')">Show people waiting here</button></TabsContent
      >
      <TabsContent value="messages"
        ><div class="observe-record-filters">
          <ScopeSelect
            v-model="channel"
            label="Channel"
            :options="[
              { value: 'all', label: 'All channels' },
              { value: 'email', label: 'Email' },
              { value: 'sms', label: 'SMS' },
              { value: 'push', label: 'Push' },
            ]"
            @update:model-value="
              channel = $event === 'all' ? '' : $event;
              emit('load', false);
            "
          /><ScopeSelect
            v-model="status"
            label="Message status"
            :options="[
              { value: 'all', label: 'All outcomes' },
              ...[
                'ACCEPTED',
                'BLOCKED',
                'FAILED',
                'UNKNOWN',
                'SKIPPED',
                'CANCELLED',
                'SCHEDULED',
                'CLAIMED',
                'DISPATCHING',
              ].map((s) => ({ value: s, label: s })),
            ]"
            @update:model-value="
              status = $event === 'all' ? '' : $event;
              emit('load', false);
            "
          />
        </div>
        <p v-if="reason" class="help-text">
          Reason: {{ reasonLabel(reason) }}
          <button
            @click="
              reason = '';
              emit('load', false);
            "
          >
            Clear reason
          </button>
        </p>
        <SkeletonBlock v-if="busy" label="Loading messages…" variant="text" />
        <p v-if="!messages.length && !busy" class="empty-inline">No matching messages.</p>
        <button v-for="m in messages" :key="m.id" class="list-row" @click="emit('open-message', m)">
          <span
            >{{ m.customer_id
            }}<small
              >{{ m.channel.toUpperCase() }} · {{ m.status }} · {{ m.attempts }} attempts</small
            ></span
          ></button
        ><button v-if="more" :disabled="busy" @click="emit('load', true)">
          Load more messages
        </button></TabsContent
      ></TabsRoot
    >
  </section>
</template>
