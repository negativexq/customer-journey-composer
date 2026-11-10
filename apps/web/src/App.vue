<script setup lang="ts">
import { computed, onMounted, onErrorCaptured, ref } from 'vue';
import { api, setIdentity, type Identity, type Journey } from './lib/api';
import AuthForm from './components/AuthForm.vue';
import JourneyList from './components/JourneyList.vue';
import JourneyEditor from './components/JourneyEditor.vue';
import SystemStatus from './components/SystemStatus.vue';
import CustomersView from './components/CustomersView.vue';
import EventsView from './components/EventsView.vue';
import SettingsView from './components/SettingsView.vue';
import TestLab from './components/TestLab.vue';
import TestInbox from './components/TestInbox.vue';
import AnalyticsView from './components/AnalyticsView.vue';
const identity = ref<Identity>();
const loading = ref(true);
const page = ref('Overview');
const selected = ref<Journey>();
const journeys = ref<Journey[]>([]);
const error = ref('');
const readOnly = computed(() => identity.value?.role === 'Viewer');
function authenticated(user: Identity) {
  identity.value = user;
  setIdentity(user);
  void refresh();
}
async function refresh() {
  try {
    journeys.value = await api<Journey[]>('/journeys?limit=200');
  } catch (cause) {
    error.value = String(cause);
  }
}
async function open(journey: Journey) {
  try {
    selected.value = await api<Journey>(`/journeys/${journey.id}`);
    page.value = 'Journeys';
  } catch (cause) {
    error.value = String(cause);
  }
}
async function openID(id: string) {
  try {
    selected.value = await api<Journey>('/journeys/' + id);
    page.value = 'Journeys';
  } catch (cause) {
    error.value = String(cause);
  }
}
async function logout() {
  try {
    await api('/auth/logout', 'POST');
    identity.value = undefined;
    setIdentity();
    selected.value = undefined;
  } catch (cause) {
    error.value = String(cause);
  }
}
function navigate(name: string) {
  page.value = name;
  selected.value = undefined;
  void refresh();
}
onMounted(async () => {
  try {
    authenticated(await api<Identity>('/auth/me'));
  } catch {
    /* Unauthenticated visitors see the sign-in form. */
  } finally {
    loading.value = false;
  }
});
onErrorCaptured((cause) => {
  error.value = String(cause);
  return false;
});
</script>
<template>
  <div v-if="loading" class="auth-page" aria-busy="true">Loading workspace…</div>
  <AuthForm v-else-if="!identity" @authenticated="authenticated" />
  <div v-else class="app-shell">
    <aside class="sidebar" aria-label="Application navigation">
      <a class="brand" href="/"
        ><span class="brand-symbol">j<span>·</span></span
        ><span>journey<span class="brand-subtitle">COMPOSER</span></span></a
      >
      <div class="workspace">
        <span class="workspace-avatar">{{ identity.workspace[0] }}</span>
        <div>
          {{ identity.workspace }}<small>{{ identity.role }}</small>
        </div>
      </div>
      <p class="nav-label">WORKSPACE</p>
      <nav>
        <button
          v-for="item in [
            'Overview',
            'Journeys',
            'Customers',
            'Events',
            'Test Lab',
            'Test Inbox',
            'Analytics',
            ...(identity.role === 'Admin' ? ['Settings'] : []),
          ]"
          :key="item"
          class="nav-item"
          :class="{ selected: page === item }"
          @click="navigate(item)"
        >
          {{ item }}
        </button>
      </nav>
      <div class="sidebar-footer">
        <div>
          {{ identity.name }}<small>{{ identity.email }}</small
          ><button @click="logout">Sign out</button>
        </div>
      </div>
    </aside>
    <div class="main-area">
      <header class="topbar">
        <span
          >{{ identity.workspace }} / <strong>{{ page }}</strong></span
        ><span class="pill">{{ identity.role }}</span>
      </header>
      <main>
        <div class="mobile-nav">
          <button @click="navigate('Overview')">Overview</button
          ><button @click="navigate('Journeys')">Journeys</button
          ><button @click="navigate('Customers')">Customers</button
          ><button @click="navigate('Events')">Events</button
          ><button @click="navigate('Analytics')">Analytics</button
          ><button @click="navigate('Test Inbox')">Test Inbox</button
          ><button @click="navigate('Test Lab')">Test Lab</button
          ><button v-if="identity.role === 'Admin'" @click="navigate('Settings')">Settings</button
          ><button @click="logout">Sign out</button>
        </div>
        <p v-if="error" role="alert" class="error-message">{{ error }}</p>
        <JourneyEditor
          v-if="selected"
          :key="selected.id"
          :journey="selected"
          :admin="identity.role === 'Admin'"
          :readonly="readOnly"
          @back="
            selected = undefined;
            refresh();
          "
          @saved="
            selected = $event;
            refresh();
          "
        /><JourneyList
          v-else-if="page === 'Journeys'"
          :readonly="readOnly"
          @open="open"
        /><CustomersView v-else-if="page === 'Customers'" :readonly="readOnly" /><EventsView
          v-else-if="page === 'Events'"
          :readonly="readOnly"
        /><TestLab v-else-if="page === 'Test Lab'" :readonly="readOnly" /><TestInbox
          v-else-if="page === 'Test Inbox'"
          :readonly="readOnly"
        /><AnalyticsView v-else-if="page === 'Analytics'" @open-journey="openID" /><SettingsView
          v-else-if="page === 'Settings'"
        /><template v-else>
          <div class="page-heading">
            <div>
              <p class="eyebrow">YOUR ORCHESTRATION WORKSPACE</p>
              <h1>Welcome, {{ identity.name }}.</h1>
              <p class="page-description">
                Every customer moment starts with a thoughtful journey.
              </p>
            </div>
            <button class="primary" @click="navigate('Journeys')">Explore journeys →</button>
          </div>
          <AnalyticsView overview @open-journey="openID" />
          <section class="panel">
            <h2>Recent journeys</h2>
            <p v-if="!journeys.length">
              Your workspace is ready. Create your first journey to get started.
            </p>
            <button
              v-for="journey in journeys.slice(0, 5)"
              :key="journey.id"
              class="list-row"
              @click="open(journey)"
            >
              <span>{{ journey.name }}</span
              ><span class="pill">{{ journey.status }}</span>
            </button>
          </section>
          <SystemStatus />
        </template>
        <footer class="page-footer">
          Customer Journey Composer · AI-generated, human-approved.
        </footer>
      </main>
    </div>
  </div>
</template>
