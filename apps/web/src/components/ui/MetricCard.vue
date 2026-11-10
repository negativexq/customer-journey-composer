<script setup lang="ts">
import AppIcon from './AppIcon.vue';
defineProps<{
  label: string;
  value: string | number;
  caption: string;
  icon: string;
  tone?: string;
}>();
defineEmits<{ select: [] }>();
</script>
<template>
  <button class="metric-card" @click="$emit('select')">
    <span class="metric-top"
      ><span>{{ label }}</span
      ><span class="metric-icon" :class="tone || 'violet'"
        ><AppIcon :name="icon" :size="17" /></span></span
    ><strong>{{ value }}</strong
    ><span class="metric-caption">{{ caption }}</span>
  </button>
</template>
