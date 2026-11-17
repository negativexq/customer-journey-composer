import test from 'node:test';
import assert from 'node:assert/strict';
import { periodDays, periodLabel, periodQuery, periodRange } from '../apps/web/src/lib/period.ts';

const now = new Date('2026-11-17T15:04:00Z');

test('a preset covers the last N UTC days including today', () => {
  assert.deepEqual(periodRange({ days: 30 }, now), { from: '2026-10-19', to: '2026-11-17' });
  assert.equal(periodDays({ days: 7 }, now).length, 7);
  assert.equal(periodQuery({ days: 7 }), 'days=7');
});

test('a custom range is inclusive and labelled for people', () => {
  const custom = { from: '2026-10-30', to: '2026-11-02' };
  assert.deepEqual(periodDays(custom, now), [
    '2026-10-30',
    '2026-10-31',
    '2026-11-01',
    '2026-11-02',
  ]);
  assert.equal(periodQuery(custom), 'from=2026-10-30&to=2026-11-02');
  assert.equal(periodLabel(custom, now), 'Oct 30 – Nov 2, 2026');
  assert.equal(
    periodLabel({ from: '2025-12-30', to: '2026-01-02' }, now),
    'Dec 30, 2025 – Jan 2, 2026',
  );
  assert.equal(periodLabel({ from: '2026-11-02', to: '2026-11-02' }, now), 'Nov 2, 2026');
});
