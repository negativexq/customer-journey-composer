import test from 'node:test';
import assert from 'node:assert/strict';
import { errorMessage, formatDateTime, plural } from '../apps/web/src/lib/format.ts';
import { diffWorkflows } from '../apps/web/src/lib/version-diff.ts';
import { stepSummary } from '../apps/web/src/lib/workflow-presentation.ts';

const step = (id, type, label, config = {}, position = { x: 0, y: 0 }) => ({
  id,
  type,
  label,
  position,
  config,
});
const summarize = (node) =>
  node.type === 'wait' ? `Wait ${node.config.duration_seconds}s` : node.type;
const base = {
  schema_version: 1,
  nodes: [
    step('start', 'event_trigger', 'Start', { event_name: 'cart_updated' }),
    step('wait', 'wait', 'Pause', { duration_seconds: 3600, timezone: 'UTC' }),
    step('mail', 'email', 'Reminder', { subject: 'Hi', html: '<p>Hi</p>' }),
  ],
  edges: [
    { id: 'e1', source: 'start', target: 'wait', port: 'started' },
    { id: 'e2', source: 'wait', target: 'mail', port: 'completed' },
  ],
  reentry: { mode: 'once' },
};

test('errors read as messages and transport failures get actionable copy', () => {
  assert.equal(errorMessage(new Error('Revision conflict')), 'Revision conflict');
  assert.equal(errorMessage('plain'), 'plain');
  assert.match(errorMessage(new TypeError('Failed to fetch')), /Could not reach the server/);
  const timeout = new Error('signal timed out');
  timeout.name = 'TimeoutError';
  assert.match(errorMessage(timeout), /took too long/);
  assert.match(errorMessage(undefined), /Something went wrong/);
});

test('counts are pluralized and dates use one explicit format', () => {
  assert.equal(plural(1, 'journey'), '1 journey');
  assert.equal(plural(0, 'journey'), '0 journeys');
  assert.equal(plural(2, 'entry', 'entries'), '2 entries');
  assert.equal(formatDateTime('2026-11-13T01:18:31Z', { timeZone: 'UTC' }), 'Nov 13, 2026, 01:18');
  assert.equal(
    formatDateTime('2026-11-13T01:18:31Z', { timeZone: 'UTC', seconds: true }),
    'Nov 13, 2026, 01:18:31',
  );
  assert.equal(formatDateTime(''), '—');
  assert.equal(formatDateTime('not a date'), '—');
});

test('version comparison ignores layout and key order but reports behavior changes', () => {
  const moved = structuredClone(base);
  moved.nodes[1].position = { x: 400, y: 900 };
  moved.nodes[1].config = { timezone: 'UTC', duration_seconds: 3600 };
  moved.edges.reverse();
  moved.edges[0].id = 'renamed-edge';
  assert.equal(diffWorkflows(base, moved, summarize).total, 0);

  const edited = structuredClone(base);
  edited.nodes[1].config.duration_seconds = 7200;
  edited.nodes[2].label = 'Second reminder';
  edited.nodes[2].config.html = '<p>Hello</p>';
  edited.nodes.push(step('exit', 'exit', 'Done', { reason: 'completed' }));
  edited.nodes.shift();
  edited.edges = [{ id: 'e2', source: 'wait', target: 'mail', port: 'completed' }];
  edited.reentry = { mode: 'after_completion' };
  edited.global_exit = { event_name: 'purchase_completed' };
  const diff = diffWorkflows(base, edited, summarize);
  assert.deepEqual(
    diff.added.map((node) => node.id),
    ['exit'],
  );
  assert.deepEqual(
    diff.removed.map((node) => node.id),
    ['start'],
  );
  assert.deepEqual(
    diff.changed.map((change) => [change.step.id, change.changes]),
    [
      ['wait', ['Wait 3600s → Wait 7200s']],
      ['mail', ['Renamed from “Reminder”', 'Settings or content edited']],
    ],
  );
  assert.equal(diff.connectionsChanged, true);
  assert.equal(diff.exitChanged, true);
  assert.equal(diff.reentryChanged, true);
  assert.equal(diff.total, 7);
});

test('waits and splits summarize what they do', () => {
  const wait = (config) => stepSummary({ type: 'wait', config });
  assert.equal(wait({ duration_seconds: 7200 }), 'Wait 2 hours');
  assert.equal(wait({ wait_mode: 'time', at_time: '09:00' }), 'Until 09:00');
  assert.equal(
    wait({ wait_mode: 'time', at_time: '18:30', weekdays: [5, 1] }),
    'Until 18:30 on Mon, Fri',
  );
  assert.equal(
    wait({
      wait_mode: 'date',
      date_path: 'customer.attributes.renews_on',
      offset_seconds: -259200,
    }),
    'Until 3 days before renews on',
  );
  assert.equal(
    wait({ wait_mode: 'date', date_path: 'event.properties.ends_at', offset_seconds: 3600 }),
    'Until 1 hour after ends at',
  );
  assert.equal(
    wait({ wait_mode: 'date', date_path: 'customer.attributes.birthday' }),
    'Until birthday',
  );
  assert.equal(stepSummary({ type: 'split', config: { percent: 30 } }), '30% A · 70% B');
});
