<script setup lang="ts">
import { metric, type Insights } from '../lib/insights';
import { channelNames, messageKinds } from '../lib/workflow';

// Messages per channel by outcome. Each count opens the messages behind it.
const props = defineProps<{ insights: Insights }>();
const emit = defineEmits<{ browse: [channel: string, status: string] }>();
const value = (name: string, channel: string) => metric(props.insights, 'channel', name, channel);
const columns = [
  { status: '', metric: 'scheduled' },
  { status: 'ACCEPTED', metric: 'ACCEPTED' },
  { status: 'BLOCKED', metric: 'BLOCKED' },
  { status: 'FAILED', metric: 'FAILED' },
  { status: 'UNKNOWN', metric: 'UNKNOWN' },
];
</script>
<template>
  <section class="surface">
    <div class="surface-heading">
      <div>
        <h3>Channel performance</h3>
        <p>Selected entry cohort · a retried send counts as one message.</p>
      </div>
    </div>
    <div class="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Channel</th>
            <th>Planned</th>
            <th>Provider accepted</th>
            <th>Blocked</th>
            <th>Failed</th>
            <th>Unknown</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in messageKinds" :key="c">
            <td>{{ channelNames[c] }}</td>
            <td v-for="column in columns" :key="column.metric">
              <button class="metric-link" @click="emit('browse', c, column.status)">
                {{ value(column.metric, c) }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="table-caption">
      Provider accepted is not confirmed delivery. Channel conversion attribution and open/click
      tracking are not available.
    </p>
  </section>
</template>
