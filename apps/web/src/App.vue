<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, onErrorCaptured, ref, watch } from 'vue';
import { pageRoute, readRoute, routeURL, type WorkspaceRoute } from './lib/navigation';
import { api, APIError, setIdentity, type Identity, type Journey } from './lib/api';
import AuthForm from './components/AuthForm.vue';
import JourneyList from './components/JourneyList.vue';
import JourneyEditor from './components/JourneyEditor.vue';
import CustomersView from './components/CustomersView.vue';
import EventsView from './components/EventsView.vue';
import SettingsView from './components/SettingsView.vue';
import TestLab from './components/TestLab.vue';
import TestInbox from './components/TestInbox.vue';
import AnalyticsView from './components/AnalyticsView.vue';
import TemplatesView from './components/TemplatesView.vue';
import OverviewDashboard from './components/OverviewDashboard.vue';
import AppIcon from './components/ui/AppIcon.vue';
import CreateJourneyDialog from './components/ui/CreateJourneyDialog.vue';
const identity = ref<Identity>();
const loading = ref(true);
const accessUnavailable = ref(false);
async function restoreSession() {
  if (loading.value && accessUnavailable.value) return;
  loading.value = true;
  accessUnavailable.value = false;
  try {
    const user = await api<Identity>('/auth/me');
    await authenticated(user);
  } catch (cause) {
    accessUnavailable.value = !(cause instanceof APIError && cause.status === 401);
  } finally {
    loading.value = false;
  }
}
const route = ref<WorkspaceRoute>(readRoute(new URL(window.location.href)));
const page = computed(() => route.value.page);
const routeLoading = ref(false);
let navigationSequence = 0;
const selected = ref<Journey>();
const journeys = ref<Journey[]>([]);
const journeyFilter = computed(() => route.value.status || '');
const error = ref('');
const showCreate = ref(false);
const readOnly = computed(() => identity.value?.role === 'Viewer');
const navGroups = computed(() => [
  {
    label: 'Workspace',
    items: [
      { name: 'Overview', icon: 'home' },
      { name: 'Journeys', icon: 'workflow' },
      { name: 'Templates', icon: 'templates' },
      { name: 'Analytics', icon: 'chart' },
    ],
  },
  {
    label: 'Audience',
    items: [
      { name: 'Customers', icon: 'users' },
      { name: 'Events', icon: 'events' },
    ],
  },
  {
    label: 'Testing',
    items: [
      { name: 'Test Lab', icon: 'flask' },
      { name: 'Test Inbox', icon: 'inbox' },
    ],
  },
]);
const searchDialog = ref<HTMLDialogElement>();
const searchInput = ref<HTMLInputElement>();
const search = ref('');
const searchResults = ref<Journey[]>([]);
const searching = ref(false);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let searchSequence = 0;
watch(search, () => {
  clearTimeout(searchTimer);
  const sequence = ++searchSequence;
  searchTimer = setTimeout(async () => {
    searching.value = true;
    try {
      const results = search.value.trim()
        ? await api<Journey[]>(
            '/journeys?limit=8&search=' + encodeURIComponent(search.value.trim()),
          )
        : journeys.value.slice(0, 8);
      if (sequence === searchSequence) searchResults.value = results;
    } catch (cause) {
      error.value = String(cause);
    } finally {
      if (sequence === searchSequence) searching.value = false;
    }
  }, 200);
});
async function showSearch() {
  search.value = '';
  searchResults.value = journeys.value.slice(0, 8);
  searchDialog.value?.showModal();
  await nextTick();
  searchInput.value?.focus();
}
function keyboard(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k' && identity.value) {
    e.preventDefault();
    void showSearch();
  }
}
async function authenticated(user: Identity) {
  identity.value = user;
  setIdentity(user);
  void refresh();
  await applyRoute(readRoute(new URL(window.location.href)));
}
async function applyRoute(next: WorkspaceRoute) {
  const request = ++navigationSequence;
  if (next.page === 'Settings' && identity.value?.role !== 'Admin') {
    next = pageRoute('Overview');
  }
  route.value = next;
  const canonical = routeURL(next);
  if (window.location.pathname + window.location.search !== canonical) {
    window.history.replaceState(null, '', canonical);
  }
  error.value = '';
  if (!next.journeyID) {
    selected.value = undefined;
    routeLoading.value = false;
    return;
  }
  if (selected.value?.id === next.journeyID) return;
  selected.value = undefined;
  routeLoading.value = true;
  try {
    const journey = await api<Journey>('/journeys/' + next.journeyID);
    if (request === navigationSequence) selected.value = journey;
  } catch (cause) {
    if (request === navigationSequence) {
      error.value = String(cause);
      route.value = pageRoute('Journeys');
      window.history.replaceState(null, '', '/journeys');
    }
  } finally {
    if (request === navigationSequence) routeLoading.value = false;
  }
}
function go(next: WorkspaceRoute) {
  const target = routeURL(next);
  if (window.location.pathname + window.location.search !== target) {
    window.history.pushState(null, '', target);
  }
  return applyRoute(next);
}
function restoreLocation() {
  if (identity.value) void applyRoute(readRoute(new URL(window.location.href)));
}
function editorNavigation(value: { tab: string; preview: string }) {
  if (route.value.journeyID) void go({ ...route.value, ...value });
}
async function refresh() {
  try {
    journeys.value = await api<Journey[]>('/journeys?limit=200');
  } catch (cause) {
    error.value = String(cause);
  }
}
async function open(j: Journey) {
  await openID(j.id);
}
async function openID(id: string) {
  searchDialog.value?.close();
  await go({ page: 'Journeys', journeyID: id, tab: 'Builder' });
}
function created(j: Journey) {
  selected.value = j;
  void go({ page: 'Journeys', journeyID: j.id, tab: 'Builder' });
  void refresh();
}
function browseJourneys(status: string) {
  void go({ ...pageRoute('Journeys'), status });
  void refresh();
}
function navigate(name: string) {
  void go(pageRoute(name));
  void refresh();
}
async function logout() {
  try {
    await api('/auth/logout', 'POST');
    identity.value = undefined;
    setIdentity();
    selected.value = undefined;
    navigationSequence++;
    route.value = pageRoute('Overview');
    window.history.replaceState(null, '', '/');
  } catch (cause) {
    error.value = String(cause);
  }
}
onMounted(async () => {
  window.addEventListener('keydown', keyboard);
  window.addEventListener('popstate', restoreLocation);
  await restoreSession();
});
onBeforeUnmount(() => {
  window.removeEventListener('keydown', keyboard);
  window.removeEventListener('popstate', restoreLocation);
  clearTimeout(searchTimer);
});
onErrorCaptured((cause) => {
  error.value = String(cause);
  return false;
});
</script>
<template>
  <div v-if="loading" class="app-loading" aria-busy="true">
    <span class="brand-mark"><AppIcon name="workflow" :size="25" /></span>
    <p>Opening your workspace…</p>
  </div>
  <main v-else-if="accessUnavailable" class="app-loading" aria-labelledby="access-title">
    <span class="brand-mark"><AppIcon name="workflow" :size="25" /></span>
    <h1 id="access-title">Workspace temporarily unavailable</h1>
    <p role="alert">We couldn't check your session. Your current page has been kept.</p>
    <button class="primary" @click="restoreSession">Try again</button>
  </main>
  <AuthForm v-else-if="!identity" @authenticated="authenticated" />
  <div v-else class="app-shell">
    <aside class="sidebar" aria-label="Application navigation">
      <a class="brand" href="/" @click.prevent="navigate('Overview')"
        ><span class="brand-mark"><AppIcon name="workflow" :size="23" /></span
        ><span>journey<span class="brand-subtitle">Composer</span></span></a
      >
      <div class="workspace">
        <span class="workspace-avatar">{{ identity.workspace[0] }}</span>
        <div>
          <strong>{{ identity.workspace }}</strong
          ><small>Workspace</small>
        </div>
        <AppIcon name="down" :size="14" />
      </div>
      <nav class="sidebar-nav">
        <section v-for="group in navGroups" :key="group.label" class="nav-group">
          <p class="nav-label">{{ group.label }}</p>
          <button
            v-for="item in group.items"
            :key="item.name"
            class="nav-item"
            :aria-label="item.name"
            :aria-current="page === item.name ? 'page' : undefined"
            :class="{ selected: page === item.name }"
            @click="navigate(item.name)"
          >
            <AppIcon :name="item.icon" :size="18" /><span>{{ item.name }}</span
            ><span v-if="item.name === 'Journeys'" class="nav-count">{{
              journeys.length === 200 ? '200+' : journeys.length
            }}</span>
          </button>
        </section>
      </nav>
      <div class="sidebar-bottom">
        <button
          v-if="identity.role === 'Admin'"
          class="nav-item"
          :class="{ selected: page === 'Settings' }"
          @click="navigate('Settings')"
        >
          <AppIcon name="settings" />Settings
        </button>
        <div class="sidebar-footer">
          <span class="user-avatar">{{
            identity.name
              .split(' ')
              .map((n) => n[0])
              .slice(0, 2)
              .join('')
          }}</span>
          <div>
            <strong>{{ identity.name }}</strong
            ><small>{{ identity.role }}</small>
          </div>
          <button class="icon-button" aria-label="Sign out" title="Sign out" @click="logout">
            <AppIcon name="logout" :size="17" />
          </button>
        </div>
      </div>
    </aside>
    <div class="main-area">
      <header class="topbar">
        <div class="breadcrumbs">
          <AppIcon name="folder" :size="16" /><span class="workspace-crumb">{{
            identity.workspace
          }}</span
          ><span class="breadcrumb-divider">/</span
          ><strong>{{ selected ? selected.name : page }}</strong>
        </div>
        <div class="topbar-actions">
          <button class="global-search" aria-label="Search journeys" @click="showSearch">
            <AppIcon name="search" :size="15" /><span>Search journeys...</span
            ><kbd>⌘ K</kbd></button
          ><span class="role-chip">{{ identity.role }}</span>
        </div>
      </header>
      <nav class="mobile-nav" aria-label="Mobile navigation">
        <button
          v-for="item in navGroups.flatMap((g) => g.items)"
          :key="item.name"
          :class="{ selected: page === item.name }"
          @click="navigate(item.name)"
        >
          <AppIcon :name="item.icon" :size="16" />{{ item.name }}</button
        ><button v-if="identity.role === 'Admin'" @click="navigate('Settings')">
          <AppIcon name="settings" />Settings</button
        ><button @click="logout">Sign out</button>
      </nav>
      <main :class="{ 'detail-page': selected }">
        <div v-if="error" role="alert" class="inline-alert error-alert">
          {{ error
          }}<button class="icon-button" aria-label="Dismiss error" @click="error = ''">
            <AppIcon name="close" />
          </button>
        </div>
        <div v-if="routeLoading" class="panel" role="status" aria-busy="true">Opening journey…</div>
        <JourneyEditor
          v-else-if="selected"
          :key="selected.id"
          :journey="selected"
          :active-tab="route.tab"
          :preview-section="route.preview"
          :admin="identity.role === 'Admin'"
          :readonly="readOnly"
          @navigate="editorNavigation"
          @back="navigate('Journeys')"
          @settings="navigate('Settings')"
          @saved="
            if (route.journeyID === $event.id) selected = $event;
            refresh();
          "
        /><JourneyList
          v-else-if="page === 'Journeys'"
          :admin="identity.role === 'Admin'"
          :initial-status="journeyFilter"
          :readonly="readOnly"
          @open="open"
          @filter="go({ page: 'Journeys', status: $event })"
          @create="showCreate = true"
          @templates="navigate('Templates')"
        /><TemplatesView
          v-else-if="page === 'Templates'"
          :readonly="readOnly"
          @open="open"
        /><CustomersView v-else-if="page === 'Customers'" :readonly="readOnly" /><EventsView
          v-else-if="page === 'Events'"
          :readonly="readOnly"
        /><TestLab v-else-if="page === 'Test Lab'" :readonly="readOnly" /><TestInbox
          v-else-if="page === 'Test Inbox'"
          :readonly="readOnly"
        /><AnalyticsView
          v-else-if="page === 'Analytics'"
          @open-journey="openID"
          @browse-journeys="browseJourneys"
          @navigate="navigate"
        /><SettingsView v-else-if="page === 'Settings'" /><OverviewDashboard
          v-else
          :name="identity.name"
          :readonly="readOnly"
          :journeys="journeys"
          @create="showCreate = true"
          @templates="navigate('Templates')"
          @open="openID"
          @browse="browseJourneys"
          @analytics="navigate('Analytics')"
        />
        <footer v-if="!selected" class="page-footer">
          <span>Customer Journey Composer</span><span>AI-generated. Human-approved.</span>
        </footer>
      </main>
    </div>
    <CreateJourneyDialog
      v-model="showCreate"
      @created="created"
      @templates="navigate('Templates')"
    />
    <dialog
      ref="searchDialog"
      class="search-dialog"
      @click="
        (e) => {
          if (e.target === searchDialog) searchDialog?.close();
        }
      "
    >
      <div class="command-search">
        <AppIcon name="search" /><input
          ref="searchInput"
          v-model="search"
          aria-label="Find journey"
          placeholder="Search journeys by name..."
        /><kbd @click="searchDialog?.close()">Esc</kbd>
      </div>
      <p class="command-label">
        {{ searching ? 'Searching...' : search ? 'MATCHING JOURNEYS' : 'RECENT JOURNEYS' }}
      </p>
      <p v-if="!searching && !searchResults.length" class="empty-command">
        No journeys found. Try another name prefix.
      </p>
      <button v-for="j in searchResults" :key="j.id" class="command-result" @click="openID(j.id)">
        <span class="journey-type-icon"><AppIcon name="workflow" /></span
        ><span
          ><strong>{{ j.name }}</strong
          ><small>{{ j.status }}</small></span
        ><AppIcon name="arrow" :size="15" />
      </button>
    </dialog>
  </div>
</template>
