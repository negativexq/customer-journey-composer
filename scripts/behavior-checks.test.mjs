import test from 'node:test';
import assert from 'node:assert/strict';
import { behaviorChecksReady, blankBehaviorChecks } from '../apps/web/src/lib/behavior-checks.ts';

test('resumed checks with omitted empty collections remain usable', () => {
  const saved = {
    version: 1,
    confirmed: true,
    entry_event: 'registered',
    email_count: 1,
    sms_count: 0,
    outcome: 'completed',
  };
  assert.equal(behaviorChecksReady(saved), true);
  assert.equal(behaviorChecksReady({ ...saved, email_count: undefined }), false);
  assert.equal(behaviorChecksReady({ ...saved, confirmed: false }), false);
  assert.equal(behaviorChecksReady(blankBehaviorChecks()), false);
});
