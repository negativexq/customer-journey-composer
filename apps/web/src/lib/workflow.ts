export const kinds = [
  'event_trigger',
  'wait',
  'wait_event',
  'condition',
  'email',
  'sms',
  'update_attribute',
  'exit',
] as const;
export type Kind = (typeof kinds)[number];
export type Rule = {
  logic?: 'and' | 'or';
  rules?: Rule[];
  path?: string;
  operator?: string;
  value?: unknown;
};
export type Config = {
  event_name?: string;
  filters?: Rule;
  duration_seconds?: number;
  timeout_seconds?: number;
  timezone?: string;
  rule?: Rule;
  subject?: string;
  preheader?: string;
  html?: string;
  text?: string;
  template_id?: string;
  fallbacks?: Record<string, string>;
  attribute?: string;
  operation?: string;
  value?: unknown;
  reason?: string;
};
export type Step = {
  id: string;
  type: Kind;
  label: string;
  position: { x: number; y: number };
  config: Config;
};
export type Workflow = {
  acceptance_contract?: string;
  schema_version: 1;
  nodes: Step[];
  edges: { id: string; source: string; target: string; port: string }[];
  global_exit?: {
    event_name: string;
    reason?: 'converted' | 'completed' | 'condition_not_met' | 'manually_cancelled';
    filters?: Rule;
  };
  reentry?: { mode: string; cooldown_seconds?: number };
};
export type Issue = { node_id?: string; field: string; message: string };
export function ports(kind: Kind): string[] {
  return {
    event_trigger: ['started'],
    wait: ['completed'],
    wait_event: ['matched', 'timeout'],
    condition: ['true', 'false'],
    email: ['next'],
    sms: ['next'],
    update_attribute: ['next'],
    exit: [],
  }[kind];
}
export function defaults(kind: Kind): Config {
  return {
    event_trigger: { event_name: 'cart_updated' },
    wait: { duration_seconds: 7200, timezone: 'UTC' },
    wait_event: { event_name: 'purchase_completed', timeout_seconds: 86400 },
    condition: { rule: { path: 'customer.attributes.purchased', operator: 'equals', value: true } },
    email: {
      subject: 'A little reminder',
      html: '<p>Hello {{ customer.attributes.first_name }}, your cart is waiting.</p>',
      text: 'Hello {{ customer.attributes.first_name }}, your cart is waiting.',
      fallbacks: { 'customer.attributes.first_name': 'there' },
    },
    sms: { text: 'Your cart is waiting.' },
    update_attribute: { attribute: 'journey_seen', operation: 'set', value: true },
    exit: { reason: 'completed' },
  }[kind];
}
export const labels: Record<Kind, string> = {
  event_trigger: 'Event Trigger',
  wait: 'Wait / Delay',
  wait_event: 'Wait for Event',
  condition: 'Condition',
  email: 'Send Email',
  sms: 'Send SMS',
  update_attribute: 'Update Attribute',
  exit: 'Exit',
};
