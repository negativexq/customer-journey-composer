<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, computed, watch } from 'vue';
import { api, type Journey } from '../lib/api';
import AppIcon from './ui/AppIcon.vue';
import StatusBadge from './ui/StatusBadge.vue';
const props = defineProps<{ readonly?: boolean; admin?: boolean; initialStatus?: string }>();
const emit = defineEmits<{ open: [Journey]; create: []; templates: [] }>();
const journeys = ref<Journey[]>([]);
const search = ref('');
const status = ref(props.initialStatus || '');
const sort = ref('updated');
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
  <section class="journeys-page">
    <div class="page-heading">
      <div>
        <h1>Journeys</h1>
        <p class="page-description">Create, test, and manage your customer experiences.</p>
      </div>
      <div class="heading-actions">
        <button @click="emit('templates')"><AppIcon name="templates" />Browse templates</button
        ><button v-if="!readonly" class="primary" @click="emit('create')">
          <AppIcon name="plus" />New journey
        </button>
      </div>
    </div>
    <div class="surface journeys-surface">
      <div class="journey-list-controls">
        <div class="list-status-tabs">
          <button
            v-for="s in ['', 'Active', 'Draft', 'Paused', 'Sunset', 'Archived']"
            :key="s"
            :class="{ active: status === s }"
            @click="status = s"
          >
            {{ s || 'All journeys' }}
          </button>
        </div>
        <div class="list-filter-row">
          <label class="search-field"
            ><AppIcon name="search" :size="17" /><input
              v-model="search"
              aria-label="Name prefix"
              type="search"
              placeholder="Search journeys by name..." /></label
          ><label class="compact-select"
            ><span>Sort by</span
            ><select v-model="sort" aria-label="Sort">
              <option value="updated">Last updated</option>
              <option value="name">Name</option>
            </select></label
          ><button class="icon-button" aria-label="Refresh journeys" @click="load()">
            <AppIcon name="refresh" :size="17" />
          </button>
        </div>
      </div>
      <p v-if="error" role="alert" class="inline-alert error-alert">{{ error }}</p>
      <div
        v-if="loading && !journeys.length"
        class="table-skeleton"
        aria-label="Loading journeys"
        aria-busy="true"
      >
        <div v-for="i in 4" :key="i" />
      </div>
      <div v-else-if="!filtered.length" class="empty-state">
        <span class="empty-icon"><AppIcon name="workflow" :size="30" /></span>
        <h2>{{ search || status ? 'No matching journeys' : 'Create your first journey' }}</h2>
        <p>
          {{
            search || status
              ? 'Try another name or status filter.'
              : 'Start from a template or describe your automation with AI.'
          }}
        </p>
        <button v-if="!readonly && !search && !status" class="primary" @click="emit('create')">
          <AppIcon name="plus" />New journey
        </button>
      </div>
      <div v-else class="table-scroll">
        <table class="journey-table">
          <thead>
            <tr>
              <th>Journey</th>
              <th>Status</th>
              <th>Performance <span class="table-meta">all time</span></th>
              <th>Version</th>
              <th>Updated</th>
              <th class="actions-column"><span class="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="j in filtered" :key="j.id">
              <td>
                <div class="journey-cell">
                  <span class="journey-type-icon"><AppIcon name="workflow" :size="19" /></span>
                  <div>
                    <button class="journey-name-button" @click="emit('open', j)">
                      {{ j.name }}
                    </button>
                    <p>{{ j.description || 'No description yet' }}</p>
                    <div v-if="j.tags.length" class="tags">
                      <span v-for="tag in j.tags.slice(0, 3)" :key="tag">{{ tag }}</span>
                    </div>
                  </div>
                </div>
              </td>
              <td><StatusBadge :status="j.status" /></td>
              <td>
                <div class="journey-counts">
                  <span
                    ><strong>{{ j.stats?.entries || 0 }}</strong
                    ><small>Entries</small></span
                  ><span
                    ><strong>{{ j.stats?.active || 0 }}</strong
                    ><small>Active</small></span
                  ><span
                    ><strong>{{ j.stats?.converted || 0 }}</strong
                    ><small>Converted</small></span
                  >
                </div>
              </td>
              <td>
                <span class="version-badge">{{ j.version ? 'v' + j.version : 'Draft' }}</span
                ><small class="table-subtext">rev {{ j.revision }}</small>
              </td>
              <td class="date-cell">
                {{
                  new Date(j.updated_at).toLocaleDateString('en-US', {
                    month: 'short',
                    day: 'numeric',
                  })
                }}<small class="table-subtext">{{
                  new Date(j.updated_at).toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit',
                  })
                }}</small>
              </td>
              <td>
                <div v-if="!readonly" class="row-actions">
                  <button
                    class="icon-button"
                    aria-label="Duplicate journey"
                    title="Duplicate journey"
                    @click="action(j, 'duplicate')"
                  >
                    <AppIcon name="copy" :size="16" /></button
                  ><button
                    v-if="admin && ['Active', 'Sunset'].includes(j.status)"
                    class="icon-button"
                    aria-label="Pause journey"
                    title="Pause"
                    @click="action(j, 'pause')"
                  >
                    <AppIcon name="pause" :size="16" /></button
                  ><button
                    v-if="admin && j.status === 'Paused'"
                    class="icon-button"
                    aria-label="Resume journey"
                    title="Resume"
                    @click="action(j, 'resume')"
                  >
                    <AppIcon name="play" :size="16" /></button
                  ><button
                    v-if="admin && ['Draft', 'Completed'].includes(j.status)"
                    class="icon-button"
                    aria-label="Archive journey"
                    title="Archive"
                    @click="action(j, 'archive')"
                  >
                    <AppIcon name="folder" :size="16" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="table-footer">
        <span
          >{{ journeys.length }} {{ more ? 'loaded' : 'journeys'
          }}<span v-if="status"> · {{ status }}</span></span
        ><button v-if="more && !loading" @click="load(true)">
          Load more journeys <AppIcon name="down" :size="14" /></button
        ><span v-else class="muted">{{ loading ? 'Loading...' : 'You’re up to date' }}</span>
      </div>
    </div>
  </section>
</template>
