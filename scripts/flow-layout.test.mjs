import test from 'node:test';
import assert from 'node:assert/strict';
import { layoutFlow } from '../apps/web/src/lib/flow-layout.ts';

const box = (id) => ({ id, width: 248, height: 180 });
const edge = (id, source, target, port) => ({ id, source, target, port });

// The recovery journey from the demo: Timeout leads to an SMS, Matched leaves the journey.
const nodes = ['cart', 'wait', 'purchase', 'sms', 'bought', 'done'].map(box);
const branches = [
  edge('a', 'cart', 'wait', 'started'),
  edge('b', 'wait', 'purchase', 'completed'),
  edge('timeout', 'purchase', 'sms', 'timeout'),
  edge('matched', 'purchase', 'bought', 'matched'),
  edge('c', 'sms', 'done', 'next'),
];

test('branch targets sit under their own handle, whatever order the edges are stored in', () => {
  for (const edges of [branches, [...branches].reverse()]) {
    const at = layoutFlow(nodes, edges);
    assert.ok(at.bought.x < at.sms.x, 'Matched (left handle) is left of Timeout (right handle)');
    assert.equal(at.done.x, at.sms.x, 'a branch keeps its own steps in one column');
    assert.ok(at.purchase.y < at.sms.y && at.sms.y < at.done.y);
  }
});

test('conditions put Yes on the left and keep shared steps in place', () => {
  const at = layoutFlow(['start', 'check', 'yes', 'no', 'join'].map(box), [
    edge('a', 'start', 'check', 'started'),
    edge('n', 'check', 'no', 'false'),
    edge('y', 'check', 'yes', 'true'),
    edge('j1', 'yes', 'join', 'next'),
    edge('j2', 'no', 'join', 'next'),
  ]);
  assert.ok(at.yes.x < at.no.x);
  assert.ok(at.join.x > at.yes.x && at.join.x < at.no.x, 'the shared step stays between them');
  const xs = Object.values(at).map((p) => `${p.x}:${p.y}`);
  assert.equal(new Set(xs).size, xs.length, 'no two steps share a position');
});
