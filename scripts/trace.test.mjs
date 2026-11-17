import test from 'node:test';
import assert from 'node:assert/strict';
import { elapsed, readableTrace } from '../apps/web/src/lib/trace.ts';

const at = (h) => `2026-11-10T${String(9 + h).padStart(2, '0')}:00:00Z`;
const trace = [
  { at: at(0), kind: 'event', message: 'ACCEPTED: cart_abandoned', execution: 0 },
  { at: at(0), kind: 'entry', node_id: 'start', message: 'Customer enrolled', execution: 1 },
  { at: at(0), kind: 'node', node_id: 'start', message: 'Entered Cart abandoned', execution: 1 },
  { at: at(0), kind: 'node', node_id: 'wait', message: 'Entered Wait 1 hour', execution: 1 },
  { at: at(0), kind: 'wait', node_id: 'wait', message: `Waiting until ${at(1)}`, execution: 1 },
  { at: at(1), kind: 'node', node_id: 'check', message: 'Entered Purchased?', execution: 1 },
  { at: at(1), kind: 'condition', node_id: 'check', message: 'Condition false', execution: 1 },
  { at: at(1), kind: 'node', node_id: 'email', message: 'Entered Your cart', execution: 1 },
  { at: at(1), kind: 'message', node_id: 'email', message: 'SIMULATED: Your cart', execution: 1 },
  { at: at(1), kind: 'node', node_id: 'end', message: 'Entered Done', execution: 1 },
  { at: at(1), kind: 'provider', node_id: 'email', message: 'Sandbox ACCEPTED', execution: 1 },
  { at: at(1), kind: 'exit', node_id: 'end', message: 'completed', execution: 1 },
];

test('folds a flat trace into readable steps', () => {
  const items = readableTrace(trace);
  assert.deepEqual(
    items.map((i) => (i.type === 'step' ? i.label : i.title)),
    [
      'Event received: cart_abandoned',
      'Cart abandoned',
      'Wait 1 hour',
      'Purchased?',
      'Your cart',
      'Done',
    ],
  );
  const [, start, wait, check, email, end] = items;
  assert.equal(start.details[0].text, 'Customer enrolled');
  assert.equal(wait.details[0].text, `Waits until ${at(1)} (1h later)`);
  assert.equal(check.details[0].text, 'Did not match: took the No path');
  // The provider answer arrives after the customer moved on but belongs to the email step.
  assert.deepEqual(
    email.details.map((d) => d.status),
    ['Simulated', 'Accepted'],
  );
  assert.equal(end.details[0].text, 'Left the journey: completed');
});

test('elapsed reads as a short schedule offset', () => {
  assert.equal(elapsed(at(0), at(0)), 'start');
  assert.equal(elapsed(at(0), '2026-11-10T09:45:00Z'), '+45m');
  assert.equal(elapsed(at(0), '2026-11-10T10:30:00Z'), '+1h 30m');
  assert.equal(elapsed(at(0), '2026-11-12T10:00:00Z'), '+2d 1h');
});
