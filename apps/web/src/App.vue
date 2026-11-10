<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { getReadiness } from './lib/api';

const state = ref<'loading' | 'ready' | 'error'>('loading');
const error = ref('');
let controller: AbortController | undefined;

async function checkStatus() {
  controller?.abort();
  const request = new AbortController();
  controller = request;
  state.value = 'loading';
  error.value = '';
  try {
    await getReadiness(AbortSignal.any([request.signal, AbortSignal.timeout(5000)]));
    if (!request.signal.aborted) state.value = 'ready';
  } catch (cause) {
    if (request.signal.aborted) return;
    state.value = 'error';
    error.value = cause instanceof Error ? cause.message : 'Unable to connect to the API.';
  }
}

onMounted(checkStatus);
onBeforeUnmount(() => controller?.abort());

const modules = [
  {
    name: 'Journey builder',
    detail: 'Design workflows with typed steps and validated connections.',
    phase: 'Phase 3',
    icon: '◇',
  },
  {
    name: 'Persistent execution',
    detail: 'Move customers through durable, event-driven journeys.',
    phase: 'Phase 5',
    icon: '↗',
  },
  {
    name: 'AI composer',
    detail: 'Draft with Claude or OpenAI, then review every change.',
    phase: 'Phase 7',
    icon: '✧',
  },
  {
    name: 'Test Lab & Inbox',
    detail: 'Advance virtual time and inspect each message and decision.',
    phase: 'Phases 8–9',
    icon: '◷',
  },
];
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" aria-label="Application navigation">
      <a class="brand" href="/" aria-label="Journey Composer home">
        <span class="brand-symbol" aria-hidden="true">j<span>·</span></span>
        <span>journey<span class="brand-subtitle">COMPOSER</span></span>
      </a>
      <div class="workspace">
        <span class="workspace-avatar">W</span>
        <div>Local workspace<small>Development environment</small></div>
      </div>
      <p class="nav-label">WORKSPACE</p>
      <nav>
        <a href="/" class="nav-item selected" aria-current="page"
          ><span aria-hidden="true">▦</span> Overview</a
        >
      </nav>
      <div class="sidebar-note">
        <span class="small-dot" /> Foundation in progress
        <p>More modules will appear as each phase is implemented.</p>
      </div>
      <div class="sidebar-footer">
        <span class="avatar">JC</span>
        <div>Customer Journey Composer<small>MVP · v0.1.0</small></div>
      </div>
    </aside>

    <div class="main-area">
      <header class="topbar">
        <span>Workspace <span class="breadcrumb-divider">/</span> <strong>Overview</strong></span
        ><span class="environment"><span class="small-dot" /> LOCAL DEVELOPMENT</span>
      </header>
      <main id="main-content">
        <div class="page-heading">
          <div>
            <p class="eyebrow">YOUR ORCHESTRATION WORKSPACE</p>
            <h1>Build journeys. Make connections.</h1>
            <p class="page-description">
              Turn customer moments into thoughtful, connected experiences.
            </p>
          </div>
          <span class="version-label">MVP FOUNDATION</span>
        </div>

        <section class="welcome-panel" aria-labelledby="welcome-heading">
          <div class="welcome-copy">
            <span class="pill">AI-generated. Human-approved.</span>
            <h2 id="welcome-heading">A clear path from<br />idea to experience.</h2>
            <p>
              Design customer workflows, test every decision, and follow each step as it happens.
            </p>
            <a class="text-link" href="/api/docs" target="_blank" rel="noopener noreferrer"
              >Explore the API <span aria-hidden="true">↗</span></a
            >
          </div>
          <div class="journey-illustration" aria-hidden="true">
            <div class="flow-node event-node">
              <span>↯</span>
              <div>Customer event<small>The journey begins</small></div>
              <i />
            </div>
            <div class="connector" />
            <div class="flow-node wait-node">
              <span>◷</span>
              <div>Wait for a moment<small>Durable, by design</small></div>
              <i />
            </div>
            <div class="connector" />
            <div class="flow-node message-node">
              <span>✉</span>
              <div>Make a connection<small>The right message</small></div>
              <i />
            </div>
          </div>
        </section>

        <div class="section-heading">
          <div>
            <h2>A foundation for what comes next</h2>
            <p>Development follows the product plan, one verified phase at a time.</p>
          </div>
          <span class="muted-label">PRODUCT ROADMAP</span>
        </div>
        <section class="module-grid" aria-label="Planned product modules">
          <article v-for="module in modules" :key="module.name" class="module-card">
            <span class="module-icon" aria-hidden="true">{{ module.icon }}</span
            ><span class="phase-label">{{ module.phase }}</span>
            <h3>{{ module.name }}</h3>
            <p>{{ module.detail }}</p>
            <span class="planned-label">Planned</span>
          </article>
        </section>

        <section class="system-panel" aria-labelledby="system-heading">
          <div class="system-info">
            <span class="system-icon" aria-hidden="true">⌁</span>
            <div>
              <h2 id="system-heading">System connection</h2>
              <p>Live readiness of the Go API and PostgreSQL schema.</p>
            </div>
          </div>
          <div class="system-actions">
            <div role="status" aria-live="polite" class="connection-status" :class="state">
              <span class="small-dot" />{{
                state === 'loading'
                  ? 'Checking services…'
                  : state === 'ready'
                    ? 'All services ready'
                    : 'Connection unavailable'
              }}
            </div>
            <button :disabled="state === 'loading'" @click="checkStatus">Check again</button>
          </div>
          <p v-if="error" role="alert" class="error-message">{{ error }}</p>
        </section>
        <footer class="page-footer">
          <span>Designed with intent. Executed with certainty.</span
          ><span>Customer Journey Composer</span>
        </footer>
      </main>
    </div>
  </div>
</template>
