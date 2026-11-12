export type InsightMetric = { dimension: string; key: string; metric: string; value: number };
export type Insights = {
  scope: {
    version: number;
    days: number;
    from: string | null;
    until: string;
    basis: string;
    timezone: string;
  };
  metrics: InsightMetric[];
  daily: { day: string; metric: string; value: number }[];
  unique_customers: number | null;
  coverage_complete: boolean;
  updated_at?: string;
  source_at?: string;
  projector_last_success?: string;
  queue_oldest_at?: string;
  journey_status: string;
  overdue: number;
};
export const metric = (data: Insights | undefined, dimension: string, name: string, key = '') =>
  data?.metrics.find((m) => m.dimension === dimension && m.metric === name && m.key === key)
    ?.value || 0;
export const reasonLabel = (name: string) =>
  ({
    consent_denied: 'Channel permission missing',
    frequency_cap: 'Sending limit reached',
    missing_contact: 'Contact details missing',
    failed: 'Provider failed',
    unknown: 'Provider outcome uncertain',
    global_exit: 'Journey exit',
    execution_cancelled: 'Execution cancelled',
    journey_cancelled: 'Journey cancelled',
    converted: 'Goal reached',
    completed: 'Completed without conversion',
    condition_not_met: 'No longer eligible',
    manually_cancelled: 'Cancelled',
  })[name] || name.replaceAll('_', ' ');
