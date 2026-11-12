import type { BehaviorChecks } from './workflow';
export const blankBehaviorChecks = (): BehaviorChecks => ({
  version: 1,
  confirmed: false,
  entry_event: '',
  inputs: {},
  outcome: 'completed',
  thresholds: [],
});
export function behaviorChecksReady(c?: BehaviorChecks): boolean {
  return (
    !!c &&
    c.confirmed &&
    !!c.entry_event &&
    [c.email_count, c.sms_count].every(
      (n) => Number.isInteger(n) && Number(n) >= 0 && Number(n) <= 100,
    ) &&
    (c.thresholds || []).length <= 4 &&
    (c.thresholds || []).every((t) => !!t.path && Number.isFinite(t.value))
  );
}
