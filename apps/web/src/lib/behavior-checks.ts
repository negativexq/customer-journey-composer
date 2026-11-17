import type { BehaviorChecks, BehaviorCase } from './workflow';
export const blankBehaviorChecks = (): BehaviorChecks => ({
  version: 2,
  case_name: 'Default situation',
  conditions: [],
  consents: { email: true, sms: true, push: true },
  events: [],
  alternatives: [],
  ineligible_outcome: 'condition_not_met',
  confirmed: false,
  entry_event: '',
  inputs: {},
  outcome: 'completed',
  thresholds: [],
});
function clauseReady(
  c: Pick<
    BehaviorCase,
    'email_count' | 'sms_count' | 'push_count' | 'duration_seconds' | 'events' | 'conditions'
  >,
): boolean {
  return (
    [c.email_count, c.sms_count].every(
      (n) => Number.isInteger(n) && Number(n) >= 0 && Number(n) <= 100,
    ) &&
    (c.push_count === undefined ||
      (Number.isInteger(c.push_count) && c.push_count >= 0 && c.push_count <= 100)) &&
    (c.duration_seconds === undefined ||
      (Number.isInteger(c.duration_seconds) &&
        c.duration_seconds >= 0 &&
        c.duration_seconds <= 31536000)) &&
    (c.events || []).length <= 8 &&
    (c.events || []).every(
      (e, i, events) =>
        !!e.event_name &&
        Number.isInteger(e.after_seconds) &&
        e.after_seconds >= 0 &&
        e.after_seconds <= 31536000 &&
        (!e.entry_binding ||
          (!!e.entry_binding.entry_path &&
            !!e.entry_binding.event_path &&
            ['equals', 'not_equals'].includes(e.entry_binding.operator))) &&
        (!i || e.after_seconds >= events[i - 1]!.after_seconds),
    ) &&
    (c.conditions || []).length <= 8 &&
    (c.conditions || []).every((r) => !!r.path && !!r.operator)
  );
}
export function behaviorChecksReady(c?: BehaviorChecks): boolean {
  return (
    !!c &&
    (c.version === 1 || c.version === 2) &&
    c.confirmed &&
    !!c.entry_event &&
    clauseReady(c) &&
    (c.version === 1 ||
      (!!c.case_name?.trim() &&
        (c.alternatives || []).length <= 7 &&
        (c.alternatives || []).every((a) => !!a.name.trim() && clauseReady(a)))) &&
    (c.thresholds || []).length <= 4 &&
    (c.thresholds || []).every((t) => !!t.path && Number.isFinite(t.value))
  );
}
