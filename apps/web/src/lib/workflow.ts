export const kinds = [
  'event_trigger',
  'wait',
  'wait_event',
  'condition',
  'split',
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
  // Share of customers a split sends to variant A.
  percent?: number;
};
export type Step = {
  id: string;
  type: Kind;
  label: string;
  position: { x: number; y: number };
  config: Config;
};
export type BehaviorChecks = {
  version: 1 | 2;
  confirmed: boolean;
  entry_event: string;
  inputs: Record<string, string | number | boolean | null>;
  email_count?: number;
  sms_count?: number;
  outcome: 'completed' | 'converted' | 'condition_not_met' | 'manually_cancelled';
  duration_seconds?: number;
  thresholds: { path: string; operator: string; value: number }[];
  case_name?: string;
  conditions?: Rule[];
  consents?: Record<string, boolean>;
  events?: BehaviorEvent[];
  expected_attributes?: Record<string, string | number | boolean | null>;
  ineligible_outcome?: 'completed' | 'condition_not_met' | 'manually_cancelled';
  alternatives?: BehaviorCase[];
};
export type BehaviorEvent = {
  entry_binding?: { entry_path: string; event_path: string; operator: 'equals' | 'not_equals' };
  event_name: string;
  after_seconds: number;
  properties?: Record<string, string | number | boolean | null>;
};
export type BehaviorCase = Pick<
  BehaviorChecks,
  | 'conditions'
  | 'inputs'
  | 'consents'
  | 'events'
  | 'expected_attributes'
  | 'email_count'
  | 'sms_count'
  | 'outcome'
  | 'duration_seconds'
> & { name: string };
export type Workflow = {
  behavior_checks?: BehaviorChecks;
  acceptance_contract?: string;
  schema_version: 1;
  nodes: Step[];
  edges: { id: string; source: string; target: string; port: string }[];
  global_exit?: {
    event_name: string;
    reason?: 'converted' | 'completed' | 'condition_not_met' | 'manually_cancelled';
    filters?: Rule;
    correlation?: { entry_path: string; event_path: string };
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
    split: ['a', 'b'],
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
    split: { percent: 50 },
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
  split: 'A/B Split',
  email: 'Send Email',
  sms: 'Send SMS',
  update_attribute: 'Update Attribute',
  exit: 'Exit',
};
