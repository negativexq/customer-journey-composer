<script setup lang="ts">
defineProps<{ name: string; size?: number }>();
const paths: Record<string, string> = {
  home: 'm3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z',
  workflow: 'M8 3H3v5h5V3Zm13 13h-5v5h5v-5ZM8 5h7a3 3 0 0 1 3 3v8M5 8v7a3 3 0 0 0 3 3h8',
  templates: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  users:
    'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75',
  user: 'M20 21v-2a7 7 0 0 0-14 0v2M13 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8',
  events: 'm3 12 4-7 5 14 5-14 4 7',
  chart: 'M3 3v18h18M7 14l4-5 4 3 5-7',
  flask: 'M9 3h6M10 3v6l-6 10a1.3 1.3 0 0 0 1 2h14a1.3 1.3 0 0 0 1-2L14 9V3M7 15h10',
  inbox: 'M4 3h16l2 12v6H2v-6L4 3Zm-2 12h6l2 3h4l2-3h6',
  settings:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M10 2h4l1 3 3 1 3-1 2 4-2 2v3l2 2-2 4-3-1-3 1-1 3h-4l-1-3-3-1-3 1-2-4 2-2v-3L1 9l2-4 3 1 3-1 1-3Z',
  search: 'M21 21l-4.5-4.5M10.5 18a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15',
  plus: 'M12 5v14M5 12h14',
  minus: 'M5 12h14',
  lock: 'M6 10h12v11H6zM8 10V6a4 4 0 0 1 8 0v4M12 15v2',
  unlock: 'M6 10h12v11H6zM8 10V6a4 4 0 0 1 7-2.6M12 15v2',
  arrow: 'M5 12h14m-6-6 6 6-6 6',
  back: 'M19 12H5m6-6-6 6 6 6',
  down: 'm6 9 6 6 6-6',
  close: 'm6 6 12 12M6 18 18 6',
  check: 'm5 12 4 4L19 6',
  sparkles: 'm12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3ZM20 2v4M18 4h4',
  clock: 'M12 7v5l3 2M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20',
  branch: 'M6 3v12a4 4 0 0 0 4 4h8M6 8h8a4 4 0 0 0 4-4V3M16 17l2 2-2 2M4 5l2-2 2 2',
  email: 'M3 5h18v14H3V5Zm0 1 9 7 9-7',
  message: 'M21 15a3 3 0 0 1-3 3H8l-5 4V5a3 3 0 0 1 3-3h12a3 3 0 0 1 3 3v10ZM7 7h10M7 12h7',
  zap: 'm13 2-9 12h7l-1 8 10-12h-8l1-8Z',
  play: 'm8 4 12 8-12 8V4Z',
  pause: 'M7 4h3v16H7zM14 4h3v16h-3z',
  exit: 'M9 3H3v18h6M9 12h12m-5-5 5 5-5 5',
  copy: 'M9 9h12v12H9zM15 9V3H3v12h6',
  paste: 'M9 5H5v16h14V5h-4M9 3h6v4H9z',
  undo: 'M3 10h11a6 6 0 0 1 0 12M3 10l5-5M3 10l5 5',
  redo: 'M21 10H10a6 6 0 0 0 0 12M21 10l-5-5M21 10l-5 5',
  layout: 'M3 3h7v7H3zM14 3h7v7h-7zM8 16h8v5H8zM6 10v3h12v-3M12 13v3',
  maximize: 'M8 3H3v5M16 3h5v5M3 16v5h5M21 16v5h-5',
  select: 'M3 8V3h5M16 3h5v5M3 16v5h5M21 16v5h-5M8 8h8v8H8z',
  file: 'M14 2H4v20h16V8l-6-6Zm0 0v6h6M8 13h8M8 17h6',
  history: 'M3 12a9 9 0 1 0 3-6.7M3 3v6h6M12 7v5l3 2',
  upload: 'M12 16V3m-5 5 5-5 5 5M4 16v5h16v-5',
  more: 'M5 12h.01M12 12h.01M19 12h.01',
  shield: 'm12 2 9 4v6c0 5-9 10-9 10S3 17 3 12V6l9-4Zm-4 10 3 3 5-6',
  logout: 'M9 3H3v18h6M9 12h12m-5-5 5 5-5 5',
  folder: 'M3 5h6l2 3h10v12H3V5Z',
  calendar: 'M4 5h16v16H4zM8 2v6M16 2v6M4 11h16',
  sun: 'M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  moon: 'M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5Z',
  monitor: 'M3 4h18v12H3zM8 21h8M12 16v5',
  refresh: 'M20 7a9 9 0 0 0-15-2L2 8m0-5v5h5M4 17a9 9 0 0 0 15 2l3-3m0 5v-5h-5',
};
</script>
<template>
  <svg
    :width="size || 18"
    :height="size || 18"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] || paths.workflow" />
  </svg>
</template>
