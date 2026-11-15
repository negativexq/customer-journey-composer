import { labels, type Rule, type Step } from './workflow.ts';

export const portLabel = (port: string) =>
  (
    ({
      true: 'Yes',
      false: 'No',
      matched: 'Matched',
      timeout: 'Timeout',
      a: 'Variant A',
      b: 'Variant B',
    }) as Record<string, string>
  )[port] ||
  (port === 'else' ? 'Otherwise' : port.startsWith('case_') ? `Branch ${port.slice(5)}` : 'Next');

export function durationLabel(seconds: number) {
  const [amount, unit] =
    seconds >= 86400
      ? [seconds / 86400, 'day']
      : seconds >= 3600
        ? [seconds / 3600, 'hour']
        : seconds >= 60
          ? [seconds / 60, 'minute']
          : [seconds, 'second'];
  return `${amount} ${unit}${amount === 1 ? '' : 's'}`;
}

export function ruleSummary(rule?: Rule, depth = 0): string {
  if (!rule || depth > 8) return 'Configure conditions';
  if (rule.rules?.length) {
    const parts = rule.rules.map((r) => ruleSummary(r, depth + 1));
    if (parts.length === 1) return parts[0]!;
    return `(${parts.join(rule.logic === 'or' ? ' OR ' : ' AND ')})`;
  }
  const field = rule.path
    ?.replace('customer.attributes.', '')
    .replace('customer.consents.', 'consent.');
  if (!field) return 'Configure conditions';
  const operator =
    (
      {
        equals: '=',
        not_equals: '≠',
        less_than: '<',
        greater_than: '>',
        greater_than_or_equal: '≥',
        less_than_or_equal: '≤',
        exists: 'exists',
        contains: 'contains',
      } as Record<string, string>
    )[rule.operator || ''] || rule.operator;
  return `${field} ${operator || ''}${rule.operator === 'exists' ? '' : ' ' + JSON.stringify(rule.value ?? '')}`;
}

function templateSummary(text: string) {
  return text.replace(/{{\s*([a-zA-Z_][\w.]*)\s*}}/g, (_match, path: string) => {
    const field = path.split('.').at(-1)!.replaceAll('_', ' ');
    return `[${field.charAt(0).toUpperCase()}${field.slice(1)}]`;
  });
}

const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
export function waitSummary(c: Step['config']): string {
  if (c.wait_mode === 'time') {
    const days = [...(c.weekdays || [])].sort((a, b) => a - b).map((day) => dayNames[day - 1]);
    return `Until ${c.at_time || '--:--'}${days.length ? ' on ' + days.join(', ') : ''}`;
  }
  if (c.wait_mode === 'date') {
    const field = (c.date_path || 'a date').split('.').at(-1)!.replaceAll('_', ' ');
    const offset = c.offset_seconds || 0;
    if (!offset) return `Until ${field}`;
    return `Until ${durationLabel(Math.abs(offset))} ${offset < 0 ? 'before' : 'after'} ${field}`;
  }
  return `Wait ${durationLabel(c.duration_seconds || 0)}`;
}

export function stepSummary(step: Pick<Step, 'type' | 'config'>): string {
  const c = step.config;
  switch (step.type) {
    case 'condition':
      return ruleSummary(c.rule);
    case 'switch':
      return [...(c.branches || []).map((branch) => branch.label || 'Unnamed'), 'otherwise'].join(
        ' · ',
      );
    case 'split':
      return `${c.percent || 0}% A · ${100 - (c.percent || 0)}% B`;
    case 'wait':
      return waitSummary(c);
    case 'wait_event':
      return `${c.event_name || 'Choose an event'} · up to ${durationLabel(c.timeout_seconds || 0)}`;
    case 'event_trigger':
      return c.event_name || 'Choose an event';
    case 'email':
      return c.subject ? templateSummary(c.subject) : 'Add email content';
    case 'sms':
      return c.text ? templateSummary(c.text) : 'Add message content';
    case 'exit':
      return c.reason === 'converted' ? 'Conversion goal reached' : c.reason || 'Journey complete';
    case 'update_attribute':
      return `${c.operation || 'set'} ${c.attribute || 'Choose an attribute'}`;
    default:
      return labels[step.type];
  }
}
