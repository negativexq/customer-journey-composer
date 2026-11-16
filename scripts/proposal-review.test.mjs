import test from 'node:test';
import assert from 'node:assert/strict';
import { proposalChanges } from '../apps/web/src/lib/proposal-review.ts';

const step = (id, type, label, config = {}) => ({
  id,
  type,
  label,
  position: { x: 0, y: 0 },
  config,
});
const draft = {
  schema_version: 1,
  nodes: [
    step('start', 'event_trigger', 'Cart abandoned', { event_name: 'cart_abandoned' }),
    step('wait_for_order', 'wait_event', 'Wait 2 hours for the order', {
      event_name: 'order_completed',
      timeout_seconds: 7200,
    }),
    step('consent', 'condition', 'Email consent?', {
      rule: { path: 'customer.consents.email', operator: 'equals', value: true },
    }),
    step('email', 'email', 'Reminder', { subject: 'Your cart is waiting' }),
    step('pause', 'wait', 'Wait 24 hours', { duration_seconds: 86400 }),
  ],
  edges: [],
};

test('a proposal lists before → after for each changed step and flags timing it changed', () => {
  const proposal = structuredClone(draft);
  proposal.nodes = proposal.nodes.filter((node) => node.id !== 'consent');
  proposal.nodes[1].label = 'Wait 1 hour for the order';
  proposal.nodes[1].config.timeout_seconds = 3600;
  proposal.nodes[2].config.subject = 'Still thinking it over?';
  proposal.nodes[3].config.duration_seconds = 90000;
  const { changed, behavior } = proposalChanges(draft, proposal);
  assert.deepEqual(
    changed.map((item) => [item.step.id, item.changes]),
    [
      [
        'wait_for_order',
        [
          'Renamed from “Wait 2 hours for the order”',
          'order_completed · up to 2 hours → order_completed · up to 1 hour',
        ],
      ],
      ['email', ['Your cart is waiting → Still thinking it over?']],
      ['pause', ['Wait 1 day → Wait 25 hours']],
    ],
  );
  // A rewritten email is what a message request asks for; a changed wait is not.
  assert.deepEqual(
    behavior.map((item) => item.step.id),
    ['wait_for_order', 'pause'],
  );
});

test('renaming a step alone is listed but needs no confirmation', () => {
  const proposal = structuredClone(draft);
  proposal.nodes[4].label = 'Pause for a day';
  const { changed, behavior } = proposalChanges(draft, proposal);
  assert.equal(changed.length, 1);
  assert.equal(behavior.length, 0);
});

test('durations read in the largest whole unit', async () => {
  const { durationLabel } = await import('../apps/web/src/lib/workflow-presentation.ts');
  assert.deepEqual([86400, 90000, 5400, 7200, 45, 0, 100].map(durationLabel), [
    '1 day',
    '25 hours',
    '90 minutes',
    '2 hours',
    '45 seconds',
    '0 seconds',
    '100 seconds',
  ]);
});
