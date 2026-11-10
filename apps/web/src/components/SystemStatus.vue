<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';
import { getReadiness } from '../lib/api';
const state = ref('loading');
const error = ref('');
let request: AbortController | undefined;
async function check() {
  request?.abort();
  const current = new AbortController();
  request = current;
  state.value = 'loading';
  error.value = '';
  try {
    await getReadiness(AbortSignal.any([current.signal, AbortSignal.timeout(5000)]));
    if (!current.signal.aborted) state.value = 'ready';
  } catch (cause) {
    if (!current.signal.aborted) {
      state.value = 'error';
      error.value = cause instanceof Error ? cause.message : 'Connection unavailable';
    }
  }
}
onMounted(check);
onBeforeUnmount(() => request?.abort());
</script>
<template>
  <section class="system-panel">
    <div class="system-info">
      <div>
        <h2>System connection</h2>
        <p>Service availability</p>
      </div>
    </div>
    <div class="system-actions">
      <div role="status" aria-live="polite" class="connection-status" :class="state">
        <span class="small-dot" />{{
          state === 'loading'
            ? 'Checking services…'
            : state === 'ready'
              ? 'All services ready'
              : 'Connection unavailable'
        }}
      </div>
      <button :disabled="state === 'loading'" @click="check">Check again</button>
    </div>
    <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  </section>
</template>
