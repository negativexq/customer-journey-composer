<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, type Journey } from '../lib/api';
import type { Workflow } from '../lib/workflow';
import AppIcon from './ui/AppIcon.vue';
defineProps<{ readonly?: boolean }>();
const emit = defineEmits<{ open: [Journey] }>();
const templates = ref<
  {
    id: string;
    name: string;
    description: string;
    tags: string[];
    builtin: boolean;
    node_count?: number;
  }[]
>([]);
const detail = ref<{ name: string; workflow: Workflow; brief?: string; notes?: string[] }>();
const error = ref('');
const busy = ref(false);
async function preview(id: string) {
  try {
    detail.value = await api('/templates/' + id);
  } catch (cause) {
    error.value = String(cause);
  }
}
async function create(id: string) {
  busy.value = true;
  error.value = '';
  try {
    emit('open', await api<Journey>(`/templates/${id}/create-journey`, 'POST', {}));
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
onMounted(async () => {
  try {
    templates.value = await api('/templates');
  } catch (cause) {
    error.value = String(cause);
  }
});
</script>
<template>
  <div class="page-heading">
    <div>
      <h1>Templates</h1>
      <p class="page-description">
        Start with a tested pattern or a workflow saved by your workspace. Templates create editable
        drafts.
      </p>
    </div>
  </div>
  <p v-if="error" class="error-message" role="alert">{{ error }}</p>
  <div class="journey-grid">
    <article v-for="t in templates" :key="t.id" class="panel journey-card">
      <div class="template-card-heading">
        <span class="step-type-icon email"><AppIcon name="workflow" :size="19" /></span
        ><span class="pill">{{ t.builtin ? 'Built-in' : 'Workspace template' }}</span>
      </div>
      <h2>{{ t.name }}</h2>
      <span v-if="t.node_count" class="template-node-count"
        >{{ t.node_count }} connected steps</span
      >
      <p>{{ t.description }}</p>
      <div class="tags">
        <span v-for="tag in t.tags" :key="tag">{{ tag }}</span>
      </div>
      <div class="toolbar">
        <button :disabled="busy || readonly" @click="create(t.id)">Use {{ t.name }}</button
        ><button @click="preview(t.id)">Preview workflow</button>
      </div>
    </article>
  </div>
  <section v-if="detail" class="panel">
    <h2>{{ detail.name }}</h2>
    <ul v-if="detail.notes?.length" class="template-guidance">
      <li v-for="note in detail.notes" :key="note">{{ note }}</li>
    </ul>
    <details v-if="detail.brief">
      <summary>Example AI brief</summary>
      <p>{{ detail.brief }}</p>
    </details>
    <p v-for="n in detail.workflow.nodes" :key="n.id">{{ n.label }} · {{ n.type }}</p>
    <details>
      <summary>Workflow specification</summary>
      <pre>{{ JSON.stringify(detail.workflow, null, 2) }}</pre>
    </details>
  </section>
</template>
<style scoped>
.template-card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}
.template-node-count {
  color: var(--muted);
  font-size: 12px;
  display: block;
  margin-top: 9px;
}
.template-guidance {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
  padding-left: 20px;
}

pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 600px;
  overflow: auto;
}
</style>
