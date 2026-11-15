import test from 'node:test';
import assert from 'node:assert/strict';
import { runContext, runTitle } from '../apps/web/src/lib/test-runs.ts';

test('test runs are named by scenario, journey and source', () => {
  const run = {
    id: '38cc779a-0000',
    result: 'PASS',
    execution_state: 'COMPLETED',
    journey_name: 'Abandoned Cart Recovery',
    scenario_name: 'Email and SMS · no purchase',
    draft_revision: 6,
    mode: 'FULL',
  };
  assert.equal(runTitle(run), 'Email and SMS · no purchase');
  assert.equal(runContext(run), 'Abandoned Cart Recovery · Draft rev 6');
  assert.equal(runContext({ ...run, version: 2 }), 'Abandoned Cart Recovery · Version 2');
  assert.equal(runTitle({ ...run, scenario_name: ' ', mode: 'SINGLE_NODE' }), 'Single-step test');
  assert.equal(
    runContext({ ...run, mode: 'SINGLE_NODE', draft_revision: null }),
    'Abandoned Cart Recovery · Draft · One step',
  );
});
