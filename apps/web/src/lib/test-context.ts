import type { Rule, Workflow } from './workflow';

export type InputField = { path: string; type: 'string' | 'number' | 'boolean'; source: string };
export type ScenarioDraft = {
  name: string;
  start_time: string;
  customer: Record<string, unknown>;
  entry_event: { event_id: string; event_name: string; properties: Record<string, unknown> };
  events: { after_seconds: number; event: { event_id: string; event_name: string } }[];
  attribute_changes: unknown[];
  assertions: Record<string, unknown>[];
};
export type TestContext = {
  entryName: string;
  eventNames: string[];
  fields: InputField[];
  notes: string[];
  sample: ScenarioDraft;
  suggestions: { label: string; note: string; scenario: ScenarioDraft }[];
};
const forbidden = new Set(['__proto__', 'constructor', 'prototype']);
export function pathValue(object: Record<string, unknown>, path: string): unknown {
  let value: unknown = object;
  for (const key of path.split('.')) {
    if (forbidden.has(key) || !value || typeof value !== 'object') return undefined;
    value = (value as Record<string, unknown>)[key];
  }
  return value;
}
export function setPath(object: Record<string, unknown>, path: string, value: unknown) {
  const keys = path.split('.');
  if (keys.some((key) => forbidden.has(key) || !key) || keys.length > 12) return;
  let current = object;
  for (const key of keys.slice(0, -1)) {
    const next = current[key];
    if (!next || typeof next !== 'object' || Array.isArray(next)) current[key] = {};
    current = current[key] as Record<string, unknown>;
  }
  current[keys.at(-1)!] = value;
}
export function testContext(workflow: Workflow, now = new Date().toISOString()): TestContext {
  const trigger = workflow.nodes.find((node) => node.type === 'event_trigger');
  const fields = new Map<string, InputField>();
  const notes = new Set<string>();
  const profile: Record<string, unknown> = {
    email: 'sandbox@example.invalid',
    phone: '+15005550006',
    locale: 'en',
    timezone: 'UTC',
    attributes: { first_name: 'Ada' },
    consents: { email: true, sms: true },
  };
  const properties: Record<string, unknown> = {};
  function field(path: string, value: unknown, source: string) {
    if (path.split('.').some((part) => forbidden.has(part))) return;
    if (!path.startsWith('customer.') && !path.startsWith('event.properties.')) {
      notes.add(`Review ${path} in Advanced options; it cannot be filled from a test profile.`);
      return;
    }
    const type =
      typeof value === 'boolean' ? 'boolean' : typeof value === 'number' ? 'number' : 'string';
    const existing = fields.get(path);
    if (!existing || source === 'Entry filter') fields.set(path, { path, type, source });
    const target = path.startsWith('customer.') ? profile : properties;
    const relative = path.replace(/^customer\.|^event\.properties\./, '');
    if (pathValue(target, relative) === undefined)
      setPath(
        target,
        relative,
        type === 'boolean' ? false : type === 'number' ? 0 : 'Sample value',
      );
  }
  function rules(rule: Rule | undefined, source: string, seed = false, depth = 0) {
    if (!rule || depth > 8) return;
    if (rule.rules) {
      if (seed && rule.logic === 'or')
        notes.add(
          'Entry includes OR conditions. Review the sample values against the intended audience.',
        );
      for (const child of rule.rules) rules(child, source, seed && rule.logic !== 'or', depth + 1);
      return;
    }
    if (!rule.path) return;
    field(rule.path, rule.value, source);
    if (seed && (rule.path.startsWith('customer.') || rule.path.startsWith('event.properties.'))) {
      let value = rule.value;
      if (rule.operator === 'greater_than' && typeof value === 'number') value += 1;
      else if (rule.operator === 'less_than' && typeof value === 'number') value -= 1;
      else if (!['equals', 'contains'].includes(rule.operator || '')) {
        notes.add(`Set a suitable sample for entry rule ${rule.path}.`);
        return;
      }
      if (['string', 'number', 'boolean'].includes(typeof value)) {
        const target = rule.path.startsWith('customer.') ? profile : properties;
        setPath(target, rule.path.replace(/^customer\.|^event\.properties\./, ''), value);
      }
    }
  }
  for (const node of workflow.nodes) {
    rules(node.config.rule, 'Condition');
    for (const text of [
      node.config.subject,
      node.config.preheader,
      node.config.text,
      node.config.html,
    ]) {
      if (!text) continue;
      for (const match of text.matchAll(/{{\s*([a-zA-Z_][\w.]*)\s*}}/g))
        field(match[1]!, node.config.fallbacks?.[match[1]!] || '', 'Message variable');
    }
  }
  rules(trigger?.config.filters, 'Entry filter', true);
  if (trigger?.config.filters)
    notes.add(
      'Entry samples are editable suggestions, not proof that every entry filter is satisfied.',
    );
  const entryName = trigger?.config.event_name || '';
  const sample: ScenarioDraft = {
    name: 'Entry and progression',
    start_time: now,
    customer: profile,
    entry_event: { event_id: 'sandbox_entry', event_name: entryName, properties },
    events: [],
    attribute_changes: [],
    assertions: [
      { type: 'enrollment_count', expected: 1 },
      { type: 'no_duplicate_delivery', expected: true },
    ],
  };
  const eventNames = [
    ...new Set(
      [
        workflow.global_exit?.event_name,
        ...workflow.nodes.filter((n) => n.type === 'wait_event').map((n) => n.config.event_name),
      ].filter((name): name is string => !!name),
    ),
  ];
  const suggestions: TestContext['suggestions'] = [
    {
      label: 'Entry and progression',
      note: 'Checks entry and duplicate prevention. Add the outcome you expect from the brief.',
      scenario: structuredClone(sample),
    },
  ];
  if (
    workflow.global_exit?.event_name &&
    workflow.nodes.some((n) => n.type === 'wait' || n.type === 'wait_event')
  ) {
    const scene = structuredClone(sample);
    scene.name = 'Global exit event';
    scene.events = [
      {
        after_seconds: 1,
        event: { event_id: 'sandbox_exit', event_name: workflow.global_exit.event_name },
      },
    ];
    scene.assertions.push({ type: 'exit_reason', expected: 'converted' });
    suggestions.push({
      label: scene.name,
      note: 'Sends the global exit event after one second. Confirm the profile can enter and is still active then.',
      scenario: scene,
    });
  }
  if (workflow.nodes.some((n) => n.type === 'email' || n.type === 'sms')) {
    const scene = structuredClone(sample);
    scene.name = 'No channel consent';
    scene.customer.consents = { email: false, sms: false };
    scene.assertions = [{ type: 'no_message' }, { type: 'no_duplicate_delivery', expected: true }];
    suggestions.push({
      label: scene.name,
      note: 'No message should be accepted when both channel consents are off. Review the intended exit separately.',
      scenario: scene,
    });
  }
  return {
    entryName,
    eventNames,
    fields: [...fields.values()].sort((a, b) => a.path.localeCompare(b.path)),
    notes: [...notes],
    sample,
    suggestions,
  };
}
