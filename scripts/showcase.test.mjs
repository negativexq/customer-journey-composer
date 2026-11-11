import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { localDemoURL } from './seed-product-demo.mjs';
test('showcase accepts loopback origins and rejects remote or credential-bearing targets', () => {
  for (const origin of ['http://localhost:8090', 'http://127.0.0.1:5173', 'http://[::1]:8090'])
    assert.equal(localDemoURL(origin), origin);
  for (const origin of [
    'https://example.com',
    'http://127.0.0.1.example.com',
    `http://fixture:${randomBytes(12).toString('hex')}@localhost:8090`,
    'http://localhost:8090/api',
    'file:///tmp/demo',
    'http://localhost:8090/?token=example',
  ])
    assert.throws(() => localDemoURL(origin));
});
