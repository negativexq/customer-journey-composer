import test from 'node:test';
import assert from 'node:assert/strict';
import { stepCaptions } from '../apps/web/src/lib/step-captions.ts';

const node = (id, type, config = {}) => ({
  id,
  type,
  label: id,
  config,
  position: { x: 0, y: 0 },
});
const edge = (source, target, port) => ({ id: source + target, source, target, port });

test('a journey reads top to bottom as a sentence', () => {
  const captions = stepCaptions({
    nodes: [
      node('cart', 'event_trigger', { event_name: 'cart_updated' }),
      node('wait', 'wait', { duration_seconds: 3600 }),
      node('purchase', 'wait_event', { event_name: 'purchase_completed', timeout_seconds: 86400 }),
      node('bought', 'exit'),
      node('vip', 'condition'),
      node('sms', 'sms'),
      node('email', 'email'),
      node('done', 'exit'),
    ],
    edges: [
      edge('cart', 'wait', 'started'),
      edge('wait', 'purchase', 'completed'),
      edge('purchase', 'bought', 'matched'),
      edge('purchase', 'vip', 'timeout'),
      edge('vip', 'sms', 'true'),
      edge('vip', 'email', 'false'),
      edge('sms', 'done', 'next'),
      edge('email', 'done', 'next'),
    ],
  });
  assert.deepEqual(captions, {
    cart: { text: 'Starts when', tone: 'start' },
    bought: { text: 'If purchase_completed arrives', tone: 'if' },
    vip: { text: 'Otherwise, after 1 day', tone: 'else' },
    sms: { text: 'If the condition is met', tone: 'if' },
    email: { text: 'Otherwise', tone: 'else' },
  });
});

test('a step reached by several paths states no single condition', () => {
  const captions = stepCaptions({
    nodes: [node('check', 'condition'), node('shared', 'exit')],
    edges: [edge('check', 'shared', 'true'), { ...edge('check', 'shared', 'false'), id: 'other' }],
  });
  assert.deepEqual(captions, {});
});
