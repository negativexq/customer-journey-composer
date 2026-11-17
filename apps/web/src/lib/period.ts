// A reporting period: the last N UTC days, or an inclusive custom range of UTC days.
// Keep this free of runtime imports so Node tests can load it.

export type Period = { days: number } | { from: string; to: string };

/** Matches the API's cap on a custom range. */
export const MAX_RANGE_DAYS = 366;

const DAY = 86400000;
const iso = (ms: number) => new Date(ms).toISOString().slice(0, 10);

/** Today's UTC date as YYYY-MM-DD. */
export function todayUTC(now = new Date()): string {
  return now.toISOString().slice(0, 10);
}

/** The first and last UTC day a period covers, both inclusive. */
export function periodRange(period: Period, now = new Date()): { from: string; to: string } {
  if ('from' in period) return { from: period.from, to: period.to };
  const today = Date.parse(todayUTC(now));
  return { from: iso(today - (period.days - 1) * DAY), to: iso(today) };
}

/** Every UTC day in the period, oldest first. */
export function periodDays(period: Period, now = new Date()): string[] {
  const { from, to } = periodRange(period, now);
  const out: string[] = [];
  for (let at = Date.parse(from); at <= Date.parse(to); at += DAY) out.push(iso(at));
  return out;
}

/** The query string the analytics endpoints take. */
export function periodQuery(period: Period): string {
  return 'from' in period
    ? `from=${encodeURIComponent(period.from)}&to=${encodeURIComponent(period.to)}`
    : `days=${period.days}`;
}

/** "Oct 19 – Nov 17, 2026", with both years when the range crosses one. */
export function periodLabel(period: Period, now = new Date()): string {
  const { from, to } = periodRange(period, now);
  const fmt = (day: string, year: boolean) =>
    new Date(day + 'T00:00:00Z').toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      ...(year ? { year: 'numeric' } : {}),
      timeZone: 'UTC',
    });
  if (from === to) return fmt(from, true);
  const sameYear = from.slice(0, 4) === to.slice(0, 4);
  return `${fmt(from, !sameYear)} – ${fmt(to, true)}`;
}
