<script setup lang="ts">
import { computed, ref } from 'vue';
const props = defineProps<{ data: { day: string; entries: number; converted: number }[] }>();
const hovered = ref(-1);
const width = 720,
  height = 220,
  pad = 35;
const top = computed(() => Math.max(3, ...props.data.map((d) => d.entries)));
const hasData = computed(() => props.data.some((d) => d.entries > 0));
const x = (i: number) => pad + (i * (width - pad * 2)) / Math.max(1, props.data.length - 1);
const y = (v: number) => height - pad - (v / top.value) * (height - pad * 2);
function line(key: 'entries' | 'converted') {
  return props.data.map((d, i) => (i ? 'L' : 'M') + x(i) + ',' + y(d[key])).join(' ');
}
const fill = computed(
  () => line('entries') + ` L${x(props.data.length - 1)},${height - pad} L${pad},${height - pad} Z`,
);
const date = (s: string) =>
  new Date(s + 'T00:00:00Z').toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  });
function hover(e: MouseEvent) {
  const box = (e.currentTarget as SVGElement).getBoundingClientRect();
  const px = ((e.clientX - box.left) / box.width) * width;
  hovered.value = Math.max(
    0,
    Math.min(
      props.data.length - 1,
      Math.round(((px - pad) / (width - pad * 2)) * (props.data.length - 1)),
    ),
  );
}
</script>
<template>
  <div class="trend-chart">
    <svg
      :viewBox="`0 0 ${width} ${height}`"
      role="img"
      aria-label="Journey entries and conversions by day"
      @mousemove="hover"
      @mouseleave="hovered = -1"
    >
      <defs>
        <linearGradient id="entries-fill" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stop-color="#7164e8" stop-opacity=".16" />
          <stop offset="100%" stop-color="#7164e8" stop-opacity=".01" />
        </linearGradient>
      </defs>
      <g v-for="i in [0, 1, 2, 3]" :key="i">
        <line
          :x1="pad"
          :x2="width - pad"
          :y1="pad + (i * (height - pad * 2)) / 3"
          :y2="pad + (i * (height - pad * 2)) / 3"
          stroke="#e9edf3"
          stroke-dasharray="3 5"
        />
        <text x="6" :y="pad + (i * (height - pad * 2)) / 3 + 4" fill="#94a0b1" font-size="10">
          {{ Math.round(top * (1 - i / 3)) }}
        </text>
      </g>
      <template v-if="hasData">
        <path :d="fill" fill="url(#entries-fill)" />
        <path :d="line('entries')" fill="none" stroke="#7564e9" stroke-width="2.5" />
        <path :d="line('converted')" fill="none" stroke="#31a18a" stroke-width="2" />
      </template>
      <text
        v-for="i in [0, Math.floor((data.length - 1) / 2), data.length - 1]"
        :key="i"
        :x="x(i)"
        :y="height - 8"
        fill="#94a0b1"
        font-size="10"
        text-anchor="middle"
      >
        {{ data[i] ? date(data[i]!.day) : '' }}
      </text>
      <g v-if="hovered >= 0 && hasData">
        <line
          :x1="x(hovered)"
          :x2="x(hovered)"
          :y1="pad"
          :y2="height - pad"
          stroke="#9a91dc"
          stroke-dasharray="4"
        />
        <circle
          :cx="x(hovered)"
          :cy="y(data[hovered]!.entries)"
          r="4"
          fill="#7564e9"
          stroke="white"
          stroke-width="2"
        />
      </g>
    </svg>
    <div v-if="!hasData" class="chart-empty">
      <span class="empty-mini-icon">↗</span><strong>No journey activity yet</strong
      ><span>Publish a journey to start seeing entries and conversions.</span>
    </div>
    <div v-else-if="hovered >= 0" class="chart-tooltip">
      {{ date(data[hovered]!.day) }} · {{ data[hovered]!.entries }} entries ·
      {{ data[hovered]!.converted }} converted
    </div>
  </div>
</template>
