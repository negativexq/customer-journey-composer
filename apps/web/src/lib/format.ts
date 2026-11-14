// Shared presentation helpers. Keep them free of runtime imports so Node tests can load this file.

/** Human-readable message for a caught value, without the "Error:" class prefix. */
export function errorMessage(cause: unknown): string {
  if (cause instanceof Error) {
    if (cause.name === 'TimeoutError')
      return 'The request took too long. Check your connection and try again.';
    if (cause.name === 'AbortError') return 'The request was cancelled.';
    if (cause.name === 'TypeError' && /fetch|network|load failed/i.test(cause.message))
      return 'Could not reach the server. Check your connection and try again.';
    return cause.message || 'Something went wrong. Please try again.';
  }
  const text = typeof cause === 'string' ? cause : '';
  return text || 'Something went wrong. Please try again.';
}

/** "1 journey", "2 journeys". */
export function plural(count: number, singular: string, pluralForm = singular + 's'): string {
  return `${count} ${count === 1 ? singular : pluralForm}`;
}

/**
 * One date-time format for lists and timelines, in the viewer's time zone:
 * "Nov 13, 2026, 01:18" (or with seconds when ordering matters).
 */
export function formatDateTime(
  value: string | number | Date | null | undefined,
  options: { seconds?: boolean; timeZone?: string } = {},
): string {
  if (value == null || value === '') return '—';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    ...(options.seconds ? { second: '2-digit' } : {}),
    hourCycle: 'h23',
    ...(options.timeZone ? { timeZone: options.timeZone } : {}),
  });
}
