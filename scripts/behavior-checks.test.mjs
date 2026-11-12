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

test('scoped contracts require valid alternate obligations and ordered event timing', () => {
  const checks = {
    ...blankBehaviorChecks(),
    confirmed: true,
    entry_event: 'entry',
    email_count: 0,
    sms_count: 0,
    alternatives: [
      {
        name: 'Event arrives',
        inputs: {},
        email_count: 1,
        sms_count: 0,
        outcome: 'completed',
        events: [{ event_name: 'ready', after_seconds: 20 }],
      },
    ],
  };
  assert.equal(behaviorChecksReady(checks), true);
  assert.equal(
    behaviorChecksReady({
      ...checks,
      alternatives: [{ ...checks.alternatives[0], email_count: undefined }],
    }),
    false,
  );
  assert.equal(
    behaviorChecksReady({
      ...checks,
      events: [
        { event_name: 'a', after_seconds: 20 },
        { event_name: 'b', after_seconds: 10 },
      ],
    }),
    false,
  );
});
