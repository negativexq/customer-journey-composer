<script setup lang="ts">
import { computed, ref } from 'vue';
import AppIcon from './ui/AppIcon.vue';
import { kinds, labels, type Kind } from '../lib/workflow';
import { descriptions, iconFor } from '../lib/step-catalog';

// Every step type, grouped and searchable. A type is added with a click or dragged onto the
// canvas.
defineProps<{ readonly?: boolean }>();
const emit = defineEmits<{ add: [Kind] }>();
const search = ref('');
const visibleKinds = computed(() =>
  kinds.filter((k) => labels[k].toLowerCase().includes(search.value.toLowerCase())),
);
const groups = computed(() =>
  [
    { label: 'Entry', kinds: ['event_trigger'] },
    { label: 'Logic & timing', kinds: ['condition', 'switch', 'split', 'wait', 'wait_event'] },
    { label: 'Messages', kinds: ['email', 'sms', 'push'] },
    { label: 'Customer & outcomes', kinds: ['update_attribute', 'webhook', 'exit'] },
  ]
    .map((group) => ({
      ...group,
      kinds: group.kinds.filter((kind) => visibleKinds.value.includes(kind as Kind)) as Kind[],
    }))
    .filter((group) => group.kinds.length),
);
function drag(event: DragEvent, kind: Kind) {
  event.dataTransfer?.setData('application/journey-node', kind);
}
</script>
<template>
  <aside class="step-library" aria-label="Step library">
    <div class="step-library-heading">
      <h2>Elements</h2>
      <span>{{ kinds.length }} types</span>
    </div>
    <label class="step-search"
      ><AppIcon name="search" :size="14" /><input
        v-model="search"
        aria-label="Search steps"
        placeholder="Find a step..."
    /></label>
    <div class="node-library">
      <section v-for="group in groups" :key="group.label" class="palette-group">
        <h4>{{ group.label }}</h4>
        <button
          v-for="kind in group.kinds"
          :key="kind"
          :draggable="!readonly"
          :disabled="readonly"
          :aria-label="'+ ' + labels[kind]"
          :class="'palette-' + kind"
          :title="descriptions[kind]"
          @dragstart="drag($event, kind)"
          @click="emit('add', kind)"
        >
          <span class="step-type-icon" :class="kind"
            ><AppIcon :name="iconFor[kind]" :size="17" /></span
          ><span
            ><strong>{{ labels[kind] }}</strong
            ><small>{{ descriptions[kind] }}</small></span
          ><AppIcon name="plus" :size="13" />
        </button>
      </section>
      <p v-if="!groups.length" class="muted">No matching steps.</p>
    </div>
    <div class="library-tip">
      <AppIcon name="sparkles" :size="16" />
      <p>Prefer to describe it?<br />Try the AI composer.</p>
    </div>
  </aside>
</template>
