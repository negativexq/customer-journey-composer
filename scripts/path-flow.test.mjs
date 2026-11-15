import test from 'node:test';
import assert from 'node:assert/strict';
import { pathFlow } from '../apps/web/src/lib/path-flow.ts';

const node = (id, type = 'email') => ({
  id,
  type,
  label: id,
  config: {},
  position: { x: 0, y: 0 },
});
const edge = (source, target, port) => ({ id: `${source}-${port}`, source, target, port });
const workflow = {
  nodes: [
    node('start', 'event_trigger'),
    node('wait', 'wait_event'),
    node('bought', 'exit'),
    node('sms', 'sms'),
    node('done', 'exit'),
    node('unused', 'exit'),
  ],
  edges: [
    edge('start', 'wait', 'started'),
    edge('wait', 'sms', 'timeout'),
    edge('wait', 'bought', 'matched'),
    edge('sms', 'done', 'next'),
  ],
};
const reached = { start: 100, wait: 100, bought: 30, sms: 70, done: 70, unused: 0 };
const taken = { 'start:started': 100, 'wait:matched': 30, 'wait:timeout': 70, 'sms:next': 70 };
const counts = {
  reached: (id) => reached[id] || 0,
  taken: (id, port) => taken[`${id}:${port}`] || 0,
};

test('steps are sized by customers and laid out left to right', () => {
  const flow = pathFlow(workflow, counts, { width: 900, height: 300, barWidth: 10, gap: 20 });
  const at = Object.fromEntries(flow.nodes.map((item) => [item.id, item]));
  assert.equal(flow.total, 100);
  assert.deepEqual(Object.keys(at).sort(), ['bought', 'done', 'sms', 'start', 'wait']);
  assert.deepEqual(
    ['start', 'wait', 'bought', 'sms', 'done'].map((id) => at[id].column),
    [0, 1, 2, 2, 3],
  );
  assert.ok(at.start.x < at.wait.x && at.wait.x < at.sms.x && at.sms.x < at.done.x);
  // Heights keep the proportion of customers.
  assert.ok(Math.abs(at.bought.height / at.sms.height - 30 / 70) < 0.01);
  assert.equal(at.start.height, at.wait.height);
  // Matched leaves above Timeout, so its target sits above too.
  assert.ok(at.bought.y < at.sms.y);
  assert.ok(flow.nodes.every((item) => item.y >= 0 && item.y + item.height <= 300.001));
});

test('bands carry the share of the step they leave', () => {
  const flow = pathFlow(workflow, counts);
  const link = Object.fromEntries(flow.links.map((item) => [`${item.source}:${item.port}`, item]));
  assert.equal(flow.links.length, 4);
  assert.equal(link['wait:matched'].value, 30);
  assert.equal(link['wait:matched'].share, 0.3);
  assert.equal(link['wait:timeout'].share, 0.7);
  assert.ok(flow.links.every((item) => /^M[\d.,-]+ C/.test(item.path) && item.path.endsWith('Z')));
});

test('a journey nobody entered has nothing to draw', () => {
  const flow = pathFlow(workflow, { reached: () => 0, taken: () => 0 });
  assert.deepEqual([flow.nodes, flow.links, flow.total], [[], [], 0]);
});
