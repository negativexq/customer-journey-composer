<script setup lang="ts">
import { ref } from 'vue';
import { api, type Identity } from '../lib/api';
import SystemStatus from './SystemStatus.vue';
const emit = defineEmits<{ authenticated: [Identity] }>();
const register = ref(false);
const email = ref('');
const password = ref('');
const name = ref('');
const workspace = ref('');
const error = ref('');
const busy = ref(false);
async function submit() {
  busy.value = true;
  error.value = '';
  try {
    emit(
      'authenticated',
      await api<Identity>(`/auth/${register.value ? 'register' : 'login'}`, 'POST', {
        email: email.value,
        password: password.value,
        ...(register.value ? { name: name.value, workspace: workspace.value } : {}),
      }),
    );
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Unable to sign in';
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <main class="auth-page">
    <a href="/" class="brand"
      ><span class="brand-symbol">j<span>·</span></span
      ><span>journey<span class="brand-subtitle">COMPOSER</span></span></a
    >
    <div class="auth-intro">
      <p class="eyebrow">AI-GENERATED. HUMAN-APPROVED.</p>
      <h1>Build journeys. Make connections.</h1>
      <p>Design, test, and follow every customer experience.</p>
    </div>
    <form class="auth-form panel" @submit.prevent="submit">
      <h2>{{ register ? 'Create your workspace' : 'Welcome back' }}</h2>
      <label v-if="register"
        >Your name<input v-model="name" autocomplete="name" required maxlength="120" /></label
      ><label v-if="register"
        >Workspace name<input v-model="workspace" required maxlength="120" /></label
      ><label
        >Email<input
          v-model="email"
          type="email"
          autocomplete="email"
          required
          maxlength="254" /></label
      ><label
        >Password<input
          v-model="password"
          type="password"
          aria-label="Password"
          aria-describedby="password-help"
          :autocomplete="register ? 'new-password' : 'current-password'"
          minlength="12"
          maxlength="72"
          required
        /><small id="password-help">At least 12 characters</small></label
      >
      <p v-if="error" role="alert" class="error-message">{{ error }}</p>
      <button class="primary" :disabled="busy">
        {{ busy ? 'Please wait…' : register ? 'Create workspace' : 'Sign in' }}</button
      ><button
        type="button"
        @click="
          register = !register;
          error = '';
        "
      >
        {{ register ? 'Already have an account? Sign in' : 'Create a workspace' }}
      </button>
    </form>
    <SystemStatus />
  </main>
</template>
