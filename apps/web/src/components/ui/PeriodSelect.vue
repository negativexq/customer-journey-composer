<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue';
import {
  PopoverAnchor,
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  RangeCalendarCell,
  RangeCalendarCellTrigger,
  RangeCalendarGrid,
  RangeCalendarGridBody,
  RangeCalendarGridHead,
  RangeCalendarGridRow,
  RangeCalendarHeadCell,
  RangeCalendarNext,
  RangeCalendarPrev,
  RangeCalendarRoot,
  type DateRange,
} from 'reka-ui';
import {
  type CalendarDate,
  endOfMonth,
  parseDate,
  startOfMonth,
  startOfYear,
  today as todayIn,
} from '@internationalized/date';
import SegmentedControl from './SegmentedControl.vue';
import AppIcon from './AppIcon.vue';
import { type Period, periodLabel, periodRange, MAX_RANGE_DAYS } from '../../lib/period';

// Three presets and a custom range of UTC days picked on a two-month calendar.
const props = defineProps<{ modelValue: Period; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [Period] }>();

const presets = [7, 30, 90];
const isPreset = (p: Period) => 'days' in p && presets.includes(p.days);
const choice = ref<number | 'custom'>(
  isPreset(props.modelValue) ? (props.modelValue as { days: number }).days : 'custom',
);
watch(
  () => props.modelValue,
  (p) => (choice.value = isPreset(p) ? (p as { days: number }).days : 'custom'),
);

const open = ref(false);
const today = todayIn('UTC') as CalendarDate;
const draft = shallowRef<DateRange>({ start: undefined, end: undefined });
function toRange(p: Period): DateRange {
  const { from, to } = periodRange(p);
  return { start: parseDate(from), end: parseDate(to) };
}
function openCalendar() {
  if (props.disabled) return;
  draft.value = toRange(props.modelValue);
  open.value = true;
}
function pick(value: unknown) {
  if (value === 'custom') openCalendar();
  else emit('update:modelValue', { days: Number(value) });
}
// Clicking the already-chosen Custom segment reopens the calendar.
function reopen(event: MouseEvent) {
  const radio = (event.target as Element).closest('[role="radio"]');
  const radios = radio?.parentElement?.querySelectorAll('[role="radio"]');
  if (radio && choice.value === 'custom' && radios && radio === radios[radios.length - 1])
    openCalendar();
}
watch(open, (now) => {
  // Closing without applying returns the control to what is actually shown.
  if (!now && !('from' in props.modelValue) && choice.value === 'custom')
    choice.value = (props.modelValue as { days: number }).days;
});

const quick = computed(() => {
  const lastMonth = startOfMonth(today).subtract({ months: 1 });
  return [
    { label: 'Last 14 days', start: today.subtract({ days: 13 }), end: today },
    { label: 'This month', start: startOfMonth(today), end: today },
    { label: 'Last month', start: lastMonth, end: endOfMonth(lastMonth) },
    { label: 'Last 3 months', start: today.subtract({ months: 3 }).add({ days: 1 }), end: today },
    { label: 'Year to date', start: startOfYear(today), end: today },
    { label: 'Last 12 months', start: today.subtract({ years: 1 }).add({ days: 1 }), end: today },
  ];
});
const sameDay = (a?: CalendarDate | null, b?: CalendarDate | null) =>
  !!a && !!b && a.compare(b) === 0;
function quickActive(q: { start: CalendarDate; end: CalendarDate }) {
  return (
    sameDay(draft.value.start as CalendarDate, q.start) &&
    sameDay(draft.value.end as CalendarDate, q.end)
  );
}
const complete = computed(() => !!draft.value.start && !!draft.value.end);
const summary = computed(() => {
  const { start, end } = draft.value;
  if (!start) return 'Choose the first day';
  if (!end) return 'Now choose the last day';
  const days = (end as CalendarDate).compare(start as CalendarDate) + 1;
  return `${periodLabel({ from: start.toString(), to: end.toString() })} · ${days} ${days === 1 ? 'day' : 'days'}`;
});
function apply() {
  const { start, end } = draft.value;
  if (!start || !end) return;
  emit('update:modelValue', { from: start.toString(), to: end.toString() });
  open.value = false;
}

const customLabel = computed(() =>
  'from' in props.modelValue ? periodLabel(props.modelValue) : 'Custom',
);
const monthTitle = (date: CalendarDate) =>
  new Date(Date.UTC(date.year, date.month - 1, 1)).toLocaleDateString('en-US', {
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  });

// One month on narrow screens, two side by side otherwise.
const months = ref(2);
let query: MediaQueryList | undefined;
const fit = () => (months.value = query?.matches ? 1 : 2);
onMounted(() => {
  query = window.matchMedia('(max-width: 640px)');
  fit();
  query.addEventListener('change', fit);
});
onBeforeUnmount(() => query?.removeEventListener('change', fit));
</script>
<template>
  <PopoverRoot v-model:open="open">
    <PopoverAnchor as-child>
      <div class="period-select" @click.capture="reopen">
        <SegmentedControl
          v-model="choice"
          class="period-segments"
          label="Trend period"
          hide-label
          :disabled="disabled"
          :options="[
            { value: 7, label: '7 days' },
            { value: 30, label: '30 days' },
            { value: 90, label: '90 days' },
            { value: 'custom', label: customLabel, icon: 'calendar' },
          ]"
          @change="pick"
        />
      </div>
    </PopoverAnchor>
    <PopoverPortal>
      <PopoverContent
        class="period-popover"
        align="end"
        :side-offset="8"
        :collision-padding="12"
        aria-label="Custom range"
      >
        <nav class="period-quick" aria-label="Quick ranges">
          <button
            v-for="q in quick"
            :key="q.label"
            type="button"
            :aria-pressed="quickActive(q)"
            @click="draft = { start: q.start, end: q.end }"
          >
            {{ q.label }}
          </button>
        </nav>
        <div class="period-calendar-wrap">
          <RangeCalendarRoot
            v-slot="{ weekDays, grid }"
            v-model="draft"
            class="period-calendar"
            locale="en-US"
            :week-starts-on="1"
            :number-of-months="months"
            :max-value="today"
            :maximum-days="MAX_RANGE_DAYS"
            :default-placeholder="draft.end || today"
            calendar-label="Custom range"
            fixed-weeks
            prevent-deselect
          >
            <div class="period-months">
              <RangeCalendarGrid
                v-for="(month, index) in grid"
                :key="month.value.toString()"
                class="period-month"
              >
                <div class="period-month-head">
                  <RangeCalendarPrev
                    v-if="index === 0"
                    class="period-nav"
                    aria-label="Previous month"
                  >
                    <AppIcon name="back" :size="15" />
                  </RangeCalendarPrev>
                  <span v-else class="period-nav-gap" />
                  <strong>{{ monthTitle(month.value as CalendarDate) }}</strong>
                  <RangeCalendarNext
                    v-if="index === grid.length - 1"
                    class="period-nav"
                    aria-label="Next month"
                  >
                    <AppIcon name="arrow" :size="15" />
                  </RangeCalendarNext>
                  <span v-else class="period-nav-gap" />
                </div>
                <RangeCalendarGridHead>
                  <RangeCalendarGridRow class="period-row">
                    <RangeCalendarHeadCell
                      v-for="day in weekDays"
                      :key="day"
                      class="period-weekday"
                    >
                      {{ day.slice(0, 2) }}
                    </RangeCalendarHeadCell>
                  </RangeCalendarGridRow>
                </RangeCalendarGridHead>
                <RangeCalendarGridBody>
                  <RangeCalendarGridRow
                    v-for="(week, w) in month.rows"
                    :key="`${month.value}-${w}`"
                    class="period-row"
                  >
                    <RangeCalendarCell
                      v-for="date in week"
                      :key="date.toString()"
                      :date="date"
                      class="period-cell"
                    >
                      <RangeCalendarCellTrigger
                        :day="date"
                        :month="month.value"
                        class="period-day"
                      />
                    </RangeCalendarCell>
                  </RangeCalendarGridRow>
                </RangeCalendarGridBody>
              </RangeCalendarGrid>
            </div>
          </RangeCalendarRoot>
          <footer class="period-footer">
            <span class="period-summary" aria-live="polite">{{ summary }}</span>
            <small>UTC days · up to {{ MAX_RANGE_DAYS }}</small>
            <div class="period-actions">
              <button type="button" @click="open = false">Cancel</button>
              <button type="button" class="primary" :disabled="!complete" @click="apply">
                Apply
              </button>
            </div>
          </footer>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
<style scoped>
.period-select {
  display: flex;
  justify-content: flex-end;
}
</style>
<style>
/* The popover is portalled to <body>, so its styles cannot be scoped. */
.period-popover {
  z-index: 60;
  display: flex;
  max-width: calc(100vw - 24px);
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  box-shadow: 0 18px 40px -12px #1a20384d;
  overflow: hidden;
  transform-origin: var(--reka-popover-content-transform-origin);
  animation: period-in 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
}
@keyframes period-in {
  from {
    opacity: 0;
    transform: translateY(-4px) scale(0.98);
  }
}
@media (prefers-reduced-motion: reduce) {
  .period-popover {
    animation: none;
  }
}
.period-quick {
  display: grid;
  align-content: start;
  gap: 2px;
  width: 148px;
  padding: 12px 8px;
  border-right: 1px solid var(--border);
  background: var(--gray-99);
}
.period-quick button {
  justify-content: flex-start;
  min-height: 34px;
  padding: 0 10px;
  border: 0;
  border-radius: 7px;
  background: none;
  color: var(--text);
  font-size: 13px;
  font-weight: 500;
  text-align: left;
}
.period-quick button:hover {
  background: var(--accent-soft);
}
.period-quick button[aria-pressed='true'] {
  background: var(--accent-soft);
  color: var(--accent-text);
  font-weight: 600;
}
.period-calendar-wrap {
  display: grid;
  min-width: 0;
}
.period-calendar {
  padding: 14px 16px 6px;
}
.period-months {
  display: flex;
  gap: 24px;
}
.period-month {
  border-collapse: collapse;
}
.period-month-head {
  display: grid;
  grid-template-columns: 30px 1fr 30px;
  align-items: center;
  margin-bottom: 8px;
  text-align: center;
}
.period-month-head strong {
  font-size: 13px;
  font-weight: 600;
}
.period-month {
  display: grid;
  align-content: start;
}
.period-nav {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  min-height: 30px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--muted);
}
.period-nav:hover {
  background: var(--gray-96);
  color: var(--text);
}
.period-nav[data-disabled] {
  opacity: 0.35;
  pointer-events: none;
}
.period-row {
  display: grid;
  grid-template-columns: repeat(7, 36px);
}
.period-weekday {
  padding-bottom: 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  text-align: center;
}
.period-cell {
  position: relative;
  padding: 1px 0;
}
.period-day {
  display: grid;
  place-items: center;
  width: 36px;
  height: 34px;
  border-radius: 8px;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--text);
  cursor: pointer;
  outline: none;
}
.period-day:hover {
  background: var(--gray-96);
}
.period-day:focus-visible {
  box-shadow: 0 0 0 2px var(--accent);
}
.period-day[data-today] {
  font-weight: 700;
  color: var(--accent-text);
}
.period-day[data-outside-view] {
  color: var(--muted);
  opacity: 0.45;
}
.period-day[data-disabled],
.period-day[data-unavailable] {
  color: var(--muted);
  opacity: 0.35;
  pointer-events: none;
}
/* The band between the two ends reads as one continuous range. */
.period-day[data-selected],
.period-day[data-highlighted] {
  border-radius: 0;
  background: var(--accent-soft);
  color: var(--accent-text);
}
.period-day[data-selection-start],
.period-day[data-selection-end],
.period-day[data-highlighted-start],
.period-day[data-highlighted-end] {
  background: var(--accent);
  color: var(--on-solid);
  font-weight: 600;
}
.period-day[data-selection-start],
.period-day[data-highlighted-start] {
  border-radius: 8px 0 0 8px;
}
.period-day[data-selection-end],
.period-day[data-highlighted-end] {
  border-radius: 0 8px 8px 0;
}
.period-day[data-selection-start][data-selection-end],
.period-day[data-highlighted-start][data-highlighted-end] {
  border-radius: 8px;
}
.period-day[data-outside-view][data-selected] {
  background: none;
  color: var(--muted);
}
.period-footer {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 2px 12px;
  padding: 10px 16px 12px;
  border-top: 1px solid var(--border);
}
.period-summary {
  font-size: 13px;
  font-weight: 600;
}
.period-footer small {
  grid-row: 2;
  font-size: 11px;
  color: var(--muted);
}
.period-actions {
  grid-row: 1 / span 2;
  grid-column: 2;
  display: flex;
  gap: 8px;
}
.period-actions button {
  min-height: 34px;
  height: 34px;
  padding: 0 14px;
}
@media (max-width: 640px) {
  .period-popover {
    flex-direction: column;
  }
  .period-quick {
    display: flex;
    width: auto;
    overflow-x: auto;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }
  .period-quick button {
    flex: none;
  }
}
</style>
