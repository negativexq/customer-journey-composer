import test from 'node:test';
import assert from 'node:assert/strict';
import { pathRows } from '../apps/web/src/lib/path-flow.ts';

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
const counts = (reached, taken, waiting = {}) => ({
  reached: (id) => reached[id] || 0,
  taken: (id, port) => taken[`${id}:${port}`] || 0,
  waiting: (id) => waiting[id] || 0,
});

test('steps read in workflow order and a split lists its branches under the step', () => {
  const { rows, summary } = pathRows(
    workflow,
    counts(
      { start: 12, wait: 12, sms: 9, done: 9 },
      { 'start:started': 12, 'wait:timeout': 9, 'sms:next': 9 },
    ),
  );
  assert.deepEqual(
    rows.map((row) => [row.kind, row.depth, row.label, row.value]),
    [
      ['step', 0, 'start', 12],
      ['step', 0, 'wait', 12],
      ['branch', 1, 'Matched', 0],
      ['step', 2, 'bought', 0],
      ['branch', 1, 'Timeout', 9],
      ['step', 2, 'sms', 9],
      ['step', 2, 'done', 9],
      ['left', 1, 'Left the journey here', 3],
    ],
  );
  assert.deepEqual(summary, {
    entered: 12,
    finished: 9,
    biggestLoss: { node: 'wait', label: 'wait', value: 3 },
  });
});

test('customers still waiting at a step are not counted as lost', () => {
  const { rows, summary } = pathRows(
    workflow,
    counts(
      { start: 10, wait: 10, sms: 6, done: 6 },
      { 'start:started': 10, 'wait:timeout': 6, 'sms:next': 6 },
      { wait: 4 },
    ),
  );
  assert.deepEqual(
    rows
      .filter((row) => row.kind === 'waiting' || row.kind === 'left')
      .map((row) => [row.kind, row.value]),
    [['waiting', 4]],
  );
  assert.equal(summary.biggestLoss, undefined);
});

test('a step reached by two paths is listed once and the second path points to it', () => {
  const merge = {
    nodes: [
      node('start', 'event_trigger'),
      node('check', 'condition'),
      node('a'),
      node('b'),
      node('end', 'exit'),
    ],
    edges: [
      edge('start', 'check', 'next'),
      edge('check', 'a', 'true'),
      edge('check', 'b', 'false'),
      edge('a', 'end', 'next'),
      edge('b', 'end', 'next'),
    ],
  };
  const { rows } = pathRows(
    merge,
    counts(
      { start: 4, check: 4, a: 3, b: 1, end: 4 },
      { 'start:next': 4, 'check:true': 3, 'check:false': 1, 'a:next': 3, 'b:next': 1 },
    ),
  );
  assert.deepEqual(
    rows.filter((row) => row.node === 'end').map((row) => row.kind),
    ['step', 'joins'],
  );
});

test('a journey nobody entered still lists its steps with zero', () => {
  const { rows, summary } = pathRows(workflow, counts({}, {}));
  assert.ok(rows.every((row) => row.value === 0));
  assert.deepEqual(summary, { entered: 0, finished: 0 });
  assert.ok(!rows.some((row) => row.node === 'unused'));
});
