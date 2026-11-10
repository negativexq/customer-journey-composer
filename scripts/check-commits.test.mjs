import assert from 'node:assert/strict';
import { test } from 'node:test';
import { validSubject } from './check-commits.mjs';

test('accepts supported types, optional scope and breaking changes', () => {
  for (const subject of [
    'feat(runtime): add durable waits',
    'ci: add checks',
    'fix(events)!: require event ids',
  ]) {
    assert.equal(validSubject(subject), true, subject);
  }
});

test('rejects vague messages, unsupported types and malformed scopes', () => {
  for (const subject of [
    'wip',
    'update',
    'feature: add timer',
    'feat(): add timer',
    'feat: ',
    'feat: Add timer',
  ]) {
    assert.equal(validSubject(subject), false, subject);
  }
});
