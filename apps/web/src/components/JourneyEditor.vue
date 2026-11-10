<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue';
import { api, type Journey } from '../lib/api';
const props = defineProps<{ journey: Journey; readonly?: boolean }>();
const emit = defineEmits<{ back: []; saved: [Journey] }>();
const name = ref(props.journey.name);
const description = ref(props.journey.description);
const tags = ref(props.journey.tags.join(', '));
const draft = ref(JSON.stringify(props.journey.draft, null, 2));
const revision = ref(props.journey.revision);
const dirty = ref(false);
const busy = ref(false);
const notice = ref('');
const error = ref('');
watch([name, description, tags, draft], () => {
  dirty.value = true;
  notice.value = 'Unsaved changes';
});
function warn(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
  }
}
window.addEventListener('beforeunload', warn);
onBeforeUnmount(() => window.removeEventListener('beforeunload', warn));
async function save() {
  busy.value = true;
  error.value = '';
  try {
    const journey = await api<Journey>(`/journeys/${props.journey.id}/draft`, 'PUT', {
      name: name.value,
      description: description.value,
      tags: tags.value
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
      draft: JSON.parse(draft.value),
      revision: revision.value,
    });
    revision.value = journey.revision;
    dirty.value = false;
    notice.value = 'All changes saved';
    emit('saved', journey);
  } catch (cause) {
    error.value = String(cause);
  } finally {
    busy.value = false;
  }
}
function back() {
  if (dirty.value && !window.confirm('Discard unsaved changes?')) return;
  emit('back');
}
</script>
<template>
  <div class="page-heading">
    <div>
      <button @click="back">← Journeys</button>
      <h1>{{ name }}</h1>
      <p class="page-description">{{ journey.status }} · Revision {{ revision }}</p>
    </div>
    <div>
      <button
        class="primary"
        :disabled="busy || readonly || journey.status === 'Archived'"
        @click="save"
      >
        Save
      </button>
      <p role="status">{{ notice }}</p>
    </div>
  </div>
  <p v-if="error" role="alert" class="error-message">{{ error }}</p>
  <section class="panel editor-form">
    <label>Name<input v-model="name" :disabled="readonly" maxlength="160" required /></label
    ><label
      >Description<textarea v-model="description" :disabled="readonly" maxlength="4000" /></label
    ><label
      >Tags<input v-model="tags" :disabled="readonly" placeholder="Comma-separated tags" /></label
    ><label
      >Workflow draft<textarea
        v-model="draft"
        :disabled="readonly"
        class="code-input"
        rows="18"
        spellcheck="false"
      />
    </label>
  </section>
</template>
