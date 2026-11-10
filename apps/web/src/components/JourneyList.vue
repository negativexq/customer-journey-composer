<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, computed, watch } from 'vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{ readonly?: boolean; admin?: boolean; initialStatus?: string }>();
const emit = defineEmits<{ open: [Journey] }>();
const journeys = ref<Journey[]>([]);
const search = ref('');
const status = ref(props.initialStatus || '');
const sort = ref('updated');
const name = ref('');
const creating = ref(false);
const loading = ref(true);
const error = ref('');
const filtered = computed(() => journeys.value);
let timer: ReturnType<typeof setTimeout> | undefined;
let sequence = 0;
const more = ref(false);
async function load(append = false) {
  const request = ++sequence;
  loading.value = true;
  error.value = '';
  try {
    const params = new URLSearchParams({
      limit: '50',
      sort: sort.value,
      search: search.value,
    });
    if (status.value) params.set('status', status.value);
    if (append && journeys.value.length) {
      const last = journeys.value.at(-1)!;
      params.set('before_id', last.id);
      if (sort.value === 'name') params.set('after_name', last.name);
      else params.set('before', last.updated_at);
    }
    const result = await api<Journey[]>('/journeys?' + params);
    if (request === sequence) {
      journeys.value = append ? [...journeys.value, ...result] : result;
      more.value = result.length === 50;
    }
  } catch (cause) {
    if (request === sequence) error.value = String(cause);
  } finally {
    if (request === sequence) loading.value = false;
  }
}
watch([search, status, sort], () => {
  clearTimeout(timer);
  timer = setTimeout(() => void load(), 250);
});
onBeforeUnmount(() => {
  clearTimeout(timer);
  sequence++;
});
async function create() {
  if (props.readonly) return;
  creating.value = true;
  error.value = '';
  try {
    const journey = await api<Journey>('/journeys', 'POST', {
      name: name.value,
      description: '',
      tags: [],
    });
    name.value = '';
    emit('open', journey);
  } catch (cause) {
    error.value = String(cause);
  } finally {
    creating.value = false;
  }
}
async function action(journey: Journey, kind: string) {
  try {
    await api(
      `/journeys/${journey.id}/${kind}`,
      'POST',
      kind === 'duplicate' ? undefined : { revision: journey.revision },
    );
    await load();
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(() => void load());
</script>
<template>
  <div class="page-heading">
    <div>
      <p class="eyebrow">WORKSPACE</p>
      <h1>Journeys</h1>
      <p class="page-description">Thoughtful experiences, from the first moment to the next.</p>
    </div>
  </div>
  <form v-if="!readonly" class="panel inline-form" @submit.prevent="create">
    <label
      >Journey name<input
        v-model="name"
        placeholder="e.g. Abandoned cart recovery"
        required
        maxlength="160" /></label
    ><button class="primary" :disabled="creating">Create journey</button>
  </form>
  <div class="toolbar">
    <label>Name prefix<input v-model="search" type="search" placeholder="Find a journey…" /></label
    ><label
      >Status<select v-model="status">
        <option value="">All statuses</option>
        <option
          v-for="s in ['Draft', 'Active', 'Paused', 'Sunset', 'Completed', 'Archived']"
          :key="s"
        >
          {{ s }}
        </option>
      </select></label
    ><label
      >Sort<select v-model="sort">
        <option value="updated">Last updated</option>
        <option value="name">Name</option>
      </select></label
    ><button @click="load()">Refresh</button>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <p v-if="loading" class="panel" aria-busy="true">Loading journeys…</p>
  <div v-else-if="!filtered.length" class="empty-state panel">
    <h2>No journeys yet</h2>
    <p>Create your first customer experience above.</p>
  </div>
  <div v-else class="journey-grid">
    <article v-for="journey in filtered" :key="journey.id" class="panel journey-card">
      <div class="card-top">
        <span class="pill">{{ journey.status }}</span
        ><small
          >Revision {{ journey.revision }} ·
          {{ journey.version ? 'v' + journey.version : 'unpublished' }}</small
        >
      </div>
      <h2>
        <button class="link-button" @click="emit('open', journey)">{{ journey.name }}</button>
      </h2>
      <p>
        {{ journey.description || 'Add a description to tell your team what this journey does.' }}
      </p>
      <div class="tags">
        <span v-for="tag in journey.tags" :key="tag">{{ tag }}</span>
      </div>
      <p v-if="journey.stats">
        {{ journey.stats.entries }} entries · {{ journey.stats.active }} active ·
        {{ journey.stats.completed }} completed · {{ journey.stats.converted }} converted
      </p>
      <small>Updated {{ new Date(journey.updated_at).toLocaleString() }}</small>
      <div v-if="!readonly" class="toolbar">
        <button @click="action(journey, 'duplicate')">Duplicate</button
        ><button
          v-if="admin && ['Draft', 'Completed'].includes(journey.status)"
          @click="action(journey, 'archive')"
        >
          Archive
        </button>
      </div>
    </article>
  </div>
  <button v-if="more && !loading" @click="load(true)">Load more journeys</button>
</template>
