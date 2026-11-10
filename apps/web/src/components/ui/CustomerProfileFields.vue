<script setup lang="ts">
import { computed, ref } from 'vue';
const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ 'update:modelValue': [string] }>();
const error = ref('');
const profile = computed(() => {
  try {
    return JSON.parse(props.modelValue) as Record<string, unknown>;
  } catch {
    return {};
  }
});
const attrs = computed(() => (profile.value.attributes || {}) as Record<string, unknown>);
const consents = computed(() => (profile.value.consents || {}) as Record<string, boolean>);
function set(key: string, value: unknown, group?: string) {
  try {
    const p = JSON.parse(props.modelValue);
    if (group) {
      p[group] = { ...(p[group] || {}), [key]: value };
    } else p[key] = value;
    emit('update:modelValue', JSON.stringify(p, null, 2));
    error.value = '';
  } catch {
    error.value = 'Correct the advanced customer JSON before editing these fields.';
  }
}
</script>
<template>
  <div class="profile-fields">
    <label
      >First name<input
        :value="attrs.first_name || ''"
        placeholder="e.g. Ada"
        @input="
          set('first_name', ($event.target as HTMLInputElement).value, 'attributes')
        " /></label
    ><label
      >Email<input
        :value="profile.email || ''"
        type="email"
        placeholder="customer@example.com"
        @input="set('email', ($event.target as HTMLInputElement).value)" /></label
    ><label
      >Phone<input
        :value="profile.phone || ''"
        placeholder="+12025550100"
        @input="set('phone', ($event.target as HTMLInputElement).value)" /></label
    ><label
      >Timezone<input
        :value="profile.timezone || 'UTC'"
        placeholder="Europe/Istanbul"
        @input="set('timezone', ($event.target as HTMLInputElement).value)"
    /></label>
    <div class="profile-consents">
      <label
        ><input
          type="checkbox"
          :checked="consents.email"
          @change="set('email', ($event.target as HTMLInputElement).checked, 'consents')"
        />Email consent</label
      ><label
        ><input
          type="checkbox"
          :checked="consents.sms"
          @change="set('sms', ($event.target as HTMLInputElement).checked, 'consents')"
        />SMS consent</label
      >
    </div>
    <p v-if="error" class="error-message">{{ error }}</p>
  </div>
</template>
