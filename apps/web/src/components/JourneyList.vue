<script setup lang="ts">
import { onMounted, ref, computed } from 'vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{ readonly?: boolean }>();
const emit = defineEmits<{ open: [Journey] }>();
const journeys = ref<Journey[]>([]);
const search = ref('');
const status = ref('');
const sort = ref('updated');
const name = ref('');
const creating = ref(false);
const loading = ref(true);
const error = ref('');
const filtered = computed(() =>
  journeys.value
    .filter(
      (j) =>
        j.name.toLowerCase().includes(search.value.toLowerCase()) &&
        (!status.value || j.status === status.value),
    )
    .sort((a, b) =>
      sort.value === 'name'
        ? a.name.localeCompare(b.name)
        : b.updated_at.localeCompare(a.updated_at),
    ),
);
async function load() {
  loading.value = true;
  error.value = '';
  try {
    journeys.value = await api<Journey[]>('/journeys?limit=200');
  } catch (cause) {
    error.value = String(cause);
  } finally {
    loading.value = false;
  }
}
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
      `/journeys/${journey.id}${kind === 'archive' ? '' : '/duplicate'}`,
      kind === 'archive' ? 'DELETE' : 'POST',
    );
    await load();
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(load);
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
    <label>Search<input v-model="search" type="search" placeholder="Find a journey…" /></label
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
    ><button @click="load">Refresh</button>
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
        ><small>Revision {{ journey.revision }}</small>
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
      <small>Updated {{ new Date(journey.updated_at).toLocaleString() }}</small>
      <div v-if="!readonly" class="toolbar">
        <button @click="action(journey, 'duplicate')">Duplicate</button
        ><button v-if="journey.status === 'Draft'" @click="action(journey, 'archive')">
          Archive
        </button>
      </div>
    </article>
  </div>
</template>
