import test from 'node:test';
import assert from 'node:assert/strict';
import { testContext, setPath } from '../apps/web/src/lib/test-context.ts';
const flow = {
  schema_version: 1,
  nodes: [
    { id: 'start', type: 'event_trigger', config: { event_name: 'user_registered' } },
    {
      id: 'check',
      type: 'condition',
      config: { rule: { path: 'customer.attributes.activated', operator: 'equals', value: true } },
    },
    {
      id: 'email',
      type: 'email',
      config: {
        subject: 'Welcome {{ customer.attributes.product_name }}',
        text: 'Hi {{ customer.attributes.first_name }}',
      },
    },
    {
      id: 'wait',
      type: 'wait_event',
      config: { event_name: 'account_activated', timeout_seconds: 172800 },
    },
    { id: 'done', type: 'exit', config: { reason: 'completed' } },
  ],
  edges: [],
  global_exit: { event_name: 'account_activated' },
};
test('workflow context uses its trigger and referenced fields, without claiming a business outcome', () => {
  const c = testContext(flow, '2026-11-12T00:00:00Z');
  assert.equal(c.sample.entry_event.event_name, 'user_registered');
  assert.equal(c.sample.customer.attributes.activated, false);
  assert.equal(c.sample.customer.attributes.product_name, 'Sample value');
  assert.deepEqual(c.eventNames, ['account_activated']);
  assert(!c.sample.assertions.some((a) => a.type === 'state'));
  assert(c.sample.assertions.some((a) => a.type === 'enrollment_count' && a.expected === 1));
  assert.equal(
    c.suggestions.find((s) => s.label === 'Global exit event').scenario.events[0].event.event_name,
    'account_activated',
  );
  assert(
    c.suggestions
      .find((s) => s.label === 'No channel consent')
      .scenario.assertions.some((a) => a.type === 'no_message'),
  );
});
test('entry values honor simple filters and nested attributes remain test-only', () => {
  const w = structuredClone(flow);
  w.nodes[0].config.filters = {
    logic: 'and',
    rules: [
      { path: 'event.properties.amount', operator: 'greater_than', value: 50 },
      { path: 'event.properties.currency', operator: 'equals', value: 'EUR' },
      { path: 'customer.attributes.plan.name', operator: 'equals', value: 'trial' },
    ],
  };
  const c = testContext(w);
  assert.equal(c.sample.entry_event.properties.amount, 51);
  assert.equal(c.sample.entry_event.properties.currency, 'EUR');
  assert.equal(c.sample.customer.attributes.plan.name, 'trial');
  assert.equal(w.nodes[0].config.filters.rules[0].value, 50);
});
test('context rejects prototype paths and flags context-only inputs instead of inventing them', () => {
  const object = {};
  setPath(object, '__proto__.polluted', true);
  setPath(object, 'constructor.prototype.polluted', true);
  assert.equal({}.polluted, undefined);
  const w = structuredClone(flow);
  w.nodes[1].config.rule = { path: 'context.score', operator: 'equals', value: 10 };
  const c = testContext(w);
  assert(c.notes.some((note) => note.includes('context.score')));
  assert.equal(c.sample.entry_event.properties.context, undefined);
});

test('a message code is not test data and a lower bound gets a passing value', () => {
  const cart = {
    schema_version: 1,
    nodes: [
      { id: 'start', type: 'event_trigger', config: { event_name: 'cart_abandoned' } },
      {
        id: 'amount',
        type: 'condition',
        config: {
          rule: { path: 'event.properties.amount', operator: 'greater_than_or_equal', value: 50 },
        },
      },
      {
        id: 'sms',
        type: 'sms',
        config: { text: 'Code {{ offer.code }}', discount_code: 'SEPET15' },
      },
    ],
    edges: [],
  };
  const context = testContext(cart, '2026-11-16T10:00:00Z');
  assert.equal(context.sample.entry_event.properties.amount, 50);
  assert.ok(!context.notes.some((note) => note.includes('offer.code')));
  assert.ok(!context.fields.some((field) => field.path === 'offer.code'));
});

test('the reviewed contract example fills the sample', () => {
  const cart = {
    schema_version: 1,
    nodes: [
      { id: 'start', type: 'event_trigger', config: { event_name: 'cart_abandoned' } },
      {
        id: 'amount',
        type: 'condition',
        config: { rule: { path: 'event.properties.amount', operator: 'greater_than', value: 50 } },
      },
    ],
    edges: [],
    behavior_checks: {
      version: 2,
      confirmed: true,
      entry_event: 'cart_abandoned',
      outcome: 'completed',
      thresholds: [],
      inputs: { 'event.properties.amount': 80, 'customer.attributes.first_name': 'Deniz' },
      consents: { sms: false },
    },
  };
  const context = testContext(cart, '2026-11-16T10:00:00Z');
  assert.equal(context.sample.entry_event.properties.amount, 80);
  assert.equal(context.sample.customer.attributes.first_name, 'Deniz');
  assert.deepEqual(context.sample.customer.consents, { email: true, sms: false });
});
