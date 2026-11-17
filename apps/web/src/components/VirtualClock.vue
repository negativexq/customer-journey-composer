<script setup lang="ts">
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
import { sandboxTime, type RunState } from '../lib/test-lab';

// The sandbox clock of a test run and the controls that move it.
defineProps<{ state: RunState; disabled: boolean }>();
const emit = defineEmits<{ advance: [operation: string, seconds?: number]; replay: [] }>();
</script>
<template>
  <section class="surface virtual-clock">
    <div>
      <span class="clock-icon"><AppIcon name="clock" :size="20" /></span>
      <div>
        <small>Virtual clock</small><strong>{{ sandboxTime(state.now) }}</strong>
      </div>
      <StatusBadge :status="state.paused ? 'PAUSED' : state.status" />
    </div>
    <div class="clock-controls">
      <button class="primary" :disabled="disabled" @click="emit('advance', 'run')">
        <AppIcon name="play" :size="14" />Run</button
      ><button :disabled="disabled" @click="emit('advance', 'step')">Step</button
      ><button :disabled="disabled" @click="emit('advance', 'advance', 3600)">Advance 1h</button
      ><button :disabled="disabled" @click="emit('advance', 'advance', 86400)">Advance 1d</button
      ><button :disabled="disabled" @click="emit('advance', 'until_next')">
        Run until next event</button
      ><button :disabled="disabled" @click="emit('advance', 'complete')">Run to completion</button
      ><button v-if="!state.paused" :disabled="disabled" @click="emit('advance', 'pause')">
        <AppIcon name="pause" :size="14" />Pause</button
      ><button v-else :disabled="disabled" @click="emit('advance', 'resume')">Resume</button
      ><button class="text-button" :disabled="disabled" @click="emit('replay')">
        <AppIcon name="refresh" :size="13" />Reset / Replay
      </button>
    </div>
  </section>
</template>
