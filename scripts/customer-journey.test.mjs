import test from 'node:test';
import assert from 'node:assert/strict';
import {
  activityFeed,
  displayName,
  initials,
  journeyPath,
  spanLabel,
} from '../apps/web/src/lib/customer-journey.ts';

const node = (id, type, label, config = {}) => ({
  id,
  type,
  label,
  config,
  position: { x: 0, y: 0 },
});
const workflow = {
  schema_version: 1,
  nodes: [
    node('cart', 'event_trigger', 'Cart updated', { event_name: 'cart_updated' }),
    node('mail', 'email', 'Reminder'),
    node('purchase', 'wait_event', 'Wait for purchase', { event_name: 'purchase_completed' }),
    node('vip', 'condition', 'Is VIP'),
    node('sms', 'sms', 'Extra incentive'),
    node('done', 'exit', 'Recovered', { reason: 'purchased' }),
  ],
  edges: [],
};
const at = (seconds) => new Date(Date.UTC(2026, 10, 13, 1, 0, seconds)).toISOString();
const step = (id, node_id, node_type, port, from, to, state = 'SUCCEEDED') => ({
  id,
  node_id,
  node_type,
  state,
  started_at: at(from),
  completed_at: to === null ? null : at(to),
  output: port ? { port } : {},
});
const execution = (overrides = {}) => ({
  id: 'run',
  journey_id: 'journey',
  version_id: 'version',
  customer_id: 'ada',
  state: 'COMPLETED',
  exit_reason: 'completed',
  current_node_id: 'sms',
  started_at: at(0),
  completed_at: at(95),
  steps: [
    step('1', 'cart', 'event_trigger', 'started', 0, 0),
    step('2', 'mail', 'email', 'next', 0, 0),
    step('3', 'purchase', 'wait_event', 'timeout', 0, 90),
    step('4', 'vip', 'condition', 'false', 90, 90),
    step('5', 'sms', 'sms', 'next', 90, 95),
  ],
  deliveries: [
    { id: 'd1', node_id: 'mail', channel: 'email', status: 'ACCEPTED' },
    {
      id: 'd2',
      node_id: 'sms',
      channel: 'sms',
      status: 'BLOCKED',
      decision: { reason: 'consent_denied' },
    },
  ],
  waiters: [],
  ...overrides,
});
const summarize = (item) => item.type;

test('a finished execution reads as the route one person took', () => {
  const path = journeyPath(workflow, execution(), summarize);
  assert.equal(path.headline, 'Finished the journey');
  assert.equal(path.duration, '1m 35s');
  assert.deepEqual(
    path.steps.map((item) => [item.label, item.outcome, item.branch, item.tone, item.took]),
    [
      ['Cart updated', 'Entered when cart_updated arrived', '', 'done', ''],
      ['Reminder', 'Email accepted by the provider', '', 'done', ''],
      [
        'Wait for purchase',
        'purchase_completed did not arrive in time',
        'Timeout',
        'done',
        '1m 30s',
      ],
      ['Is VIP', 'Condition was not met', 'No', 'done', ''],
      ['Extra incentive', 'SMS not sent: no permission for this channel', '', 'blocked', '5s'],
    ],
  );
  assert.equal(path.steps[1].deliveryID, 'd1');
  assert.deepEqual(
    path.notReached.map((item) => item.label),
    ['Recovered'],
  );
});

test('a waiting execution marks where the person is now', () => {
  const waiting = execution({
    state: 'WAITING',
    exit_reason: null,
    current_node_id: 'purchase',
    completed_at: null,
    steps: [
      step('1', 'cart', 'event_trigger', 'started', 0, 0),
      step('3', 'purchase', 'wait_event', '', 0, null, 'WAITING'),
    ],
    deliveries: [],
    waiters: [{ node_id: 'purchase', event_name: 'purchase_completed', state: 'WAITING' }],
  });
  const path = journeyPath(workflow, waiting, summarize);
  assert.equal(path.headline, 'Waiting at Wait for purchase');
  assert.equal(path.duration, '');
  assert.deepEqual(
    [path.steps[1].tone, path.steps[1].outcome],
    ['current', 'Waiting for purchase_completed'],
  );
  assert.equal(path.notReached.length, 4);
});

test('a route still reads when the definition is unavailable', () => {
  const failed = execution({ state: 'FAILED', current_node_id: 'mail' });
  failed.steps[1].state = 'FAILED';
  const path = journeyPath({ schema_version: 1, nodes: [], edges: [] }, failed, summarize);
  assert.equal(path.headline, 'Stopped by a failure at mail');
  assert.deepEqual([path.steps[1].label, path.steps[1].tone], ['mail', 'failed']);
  assert.deepEqual(path.notReached, []);
});

test('activity merges events, messages and entries newest first', () => {
  const feed = activityFeed(
    [{ event_id: 'e1', event_name: 'cart_updated', occurred_at: at(0), status: 'ACCEPTED' }],
    [{ id: 'd1', channel: 'sms', mode: 'SIMULATED', status: 'SIMULATED', created_at: at(30) }],
    [{ id: 'run', state: 'COMPLETED', started_at: at(1) }],
    () => 'Cart recovery',
  );
  assert.deepEqual(
    feed.map((item) => [item.kind, item.title, item.detail]),
    [
      ['message', 'SMS message', 'simulated'],
      ['execution', 'Entered Cart recovery', 'Journey entry'],
      ['event', 'cart_updated', 'Event received'],
    ],
  );
});

test('people are named from what the profile knows', () => {
  assert.equal(
    displayName({
      customer_id: 'c1',
      email: 'a@example.invalid',
      attributes: { first_name: 'Ada' },
    }),
    'Ada',
  );
  assert.equal(
    displayName({ customer_id: 'c1', email: 'a@example.invalid', attributes: null }),
    'a@example.invalid',
  );
  assert.equal(displayName({ customer_id: 'c1' }), 'c1');
  assert.equal(initials('oliver-taylor@example.invalid'), 'OT');
  assert.equal(initials('Ada'), 'A');
  assert.equal(initials(''), '?');
  assert.equal(spanLabel(at(0), at(3725)), '1h 2m');
  assert.equal(spanLabel(at(0), at(0)), '');
});
