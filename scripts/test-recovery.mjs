import { randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
const base = process.env.CJC_API_URL || 'http://127.0.0.1:8090';
let cookie = '',
  csrf = '';
async function call(path, method = 'GET', body) {
  const response = await fetch(`${base}/api/v1${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(10000),
  });
  if (!response.ok) throw new Error(`${path}: HTTP ${response.status}`);
  if (response.headers.has('set-cookie')) cookie = response.headers.get('set-cookie').split(';')[0];
  return response.json();
}
async function untilState(id, state) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const execution = await call(`/executions/${id}`);
    if (execution.state === state) return execution;
    assert.notEqual(execution.state, 'FAILED');
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`Execution did not reach ${state}`);
}
let stopped = false;
try {
  const identity = await call('/auth/register', 'POST', {
    name: 'Recovery check',
    workspace: 'Isolated recovery verification',
    email: `${randomBytes(12).toString('hex')}@example.invalid`,
    password: randomBytes(24).toString('hex'),
  });
  csrf = identity.csrf_token;
  const journey = await call('/journeys', 'POST', { name: 'Persistent wait recovery' });
  const workflow = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'delay',
        type: 'wait',
        label: 'Wait',
        position: { x: 0, y: 160 },
        config: { duration_seconds: 5 },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 320 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'delay', port: 'started' },
      { id: 'b', source: 'delay', target: 'done', port: 'completed' },
    ],
  };
  const saved = await call(`/journeys/${journey.id}/draft`, 'PUT', {
    name: journey.name,
    draft: workflow,
    revision: journey.revision,
  });
  await call(`/journeys/${journey.id}/publish`, 'POST', { revision: saved.revision });
  await call('/customers', 'POST', { customer_id: 'recovery_customer' });
  const event = {
    event_id: randomUUID(),
    customer_id: 'recovery_customer',
    event_name: 'cart_updated',
    occurred_at: new Date().toISOString(),
    properties: {},
  };
  const accepted = await call('/events', 'POST', event);
  assert.equal(accepted.executions.length, 1);
  await untilState(accepted.executions[0], 'WAITING');
  execFileSync('docker', ['compose', 'stop', 'worker'], { stdio: 'pipe' });
  stopped = true;
  const duplicate = await call('/events', 'POST', event);
  assert.equal(duplicate.status, 'DUPLICATE');
  assert.equal(duplicate.executions.length, 0);
  execFileSync('docker', ['compose', 'start', 'worker'], { stdio: 'pipe' });
  stopped = false;
  const completed = await untilState(accepted.executions[0], 'COMPLETED');
  assert.equal(completed.steps.length, 3);
  assert.equal(new Set(completed.steps.map((step) => step.node_id)).size, 3);
  console.log(
    'PASS: actual worker process restart preserves a durable wait; duplicate event creates no second execution.',
  );
} finally {
  if (stopped) execFileSync('docker', ['compose', 'start', 'worker'], { stdio: 'pipe' });
}
