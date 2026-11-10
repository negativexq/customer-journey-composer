import { randomBytes, randomUUID } from 'node:crypto';
import assert from 'node:assert/strict';
const base = process.env.CJC_API_URL || 'http://127.0.0.1:8090';
const prometheus = process.env.CJC_PROMETHEUS_URL || 'http://127.0.0.1:9095';
const tempo = process.env.CJC_TEMPO_URL || 'http://127.0.0.1:3205';
const traceID = randomBytes(16).toString('hex');
const parent = randomBytes(8).toString('hex');
let cookie = '',
  csrf = '';
async function call(path, method = 'GET', body) {
  const response = await fetch(base + '/api/v1' + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Cookie: cookie,
      'X-CSRF-Token': csrf,
      traceparent: `00-${traceID}-${parent}-01`,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(15000),
  });
  if (!response.ok) throw new Error(`${path}: HTTP ${response.status}`);
  if (response.headers.has('set-cookie')) cookie = response.headers.get('set-cookie').split(';')[0];
  return response.json();
}
async function until(fn, label) {
  const deadline = Date.now() + 45000;
  while (Date.now() < deadline) {
    try {
      const value = await fn();
      if (value) return value;
    } catch (error) {
      if (!(error instanceof TypeError) && error.name !== 'TimeoutError') throw error;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(label + ' timed out');
}
for (const [name, url] of [
  ['Tempo', tempo + '/ready'],
  ['Prometheus', prometheus + '/-/ready'],
]) {
  await until(
    async () => (await fetch(url, { signal: AbortSignal.timeout(5000) })).ok,
    name + ' readiness',
  );
}
const identity = await call('/auth/register', 'POST', {
  name: 'Telemetry probe',
  workspace: 'Isolated telemetry probe',
  email: randomBytes(12).toString('hex') + '@example.invalid',
  password: randomBytes(24).toString('hex'),
});
csrf = identity.csrf_token;
const j = await call('/journeys', 'POST', { name: 'Telemetry propagation' });
const workflow = {
  schema_version: 1,
  nodes: [
    {
      id: 'start',
      type: 'event_trigger',
      label: 'Event',
      config: { event_name: 'telemetry_probe' },
    },
    { id: 'sms', type: 'sms', label: 'Mock SMS', config: { text: 'Telemetry smoke test' } },
    { id: 'done', type: 'exit', label: 'Done', config: { reason: 'completed' } },
  ],
  edges: [
    { id: 'a', source: 'start', target: 'sms', port: 'started' },
    { id: 'b', source: 'sms', target: 'done', port: 'next' },
  ],
};
const saved = await call(`/journeys/${j.id}/draft`, 'PUT', {
  name: j.name,
  draft: workflow,
  revision: j.revision,
});
const scenario = {
  name: 'Telemetry publish gate',
  entry_event: { event_name: 'telemetry_probe' },
  customer: { phone: '+12025550100', consents: { sms: true } },
  assertions: [
    { type: 'message_sent', channel: 'sms' },
    { type: 'state', expected: 'COMPLETED' },
  ],
};
const run = await call('/test-runs', 'POST', { journey_id: j.id, scenario });
const completed = await call(`/test-runs/${run.id}/advance`, 'POST', {
  operation: 'complete',
  revision: run.revision,
});
assert.equal(completed.state.result, 'PASS');
await call(`/journeys/${j.id}/publish`, 'POST', { revision: saved.revision });
await call('/customers', 'POST', {
  customer_id: 'telemetry_probe',
  phone: '+12025550100',
  consents: { sms: true },
});
const entry = await call('/events', 'POST', {
  event_id: randomUUID(),
  customer_id: 'telemetry_probe',
  event_name: 'telemetry_probe',
  occurred_at: new Date().toISOString(),
  properties: {},
});
assert.equal(entry.executions.length, 1);
await until(async () => {
  const e = await call('/executions/' + entry.executions[0]);
  return e.state === 'COMPLETED' && e.deliveries?.some((d) => d.status === 'ACCEPTED');
}, 'Mock execution');
const trace = await until(async () => {
  const r = await fetch(tempo + '/api/traces/' + traceID, {
    headers: { Accept: 'application/json' },
    signal: AbortSignal.timeout(5000),
  });
  if (!r.ok) return false;
  const body = await r.text();
  return body.includes('runtime.step') && body.includes('messaging.provider.send') ? body : false;
}, 'Durable distributed trace');
assert(!trace.includes('+12025550100'));
assert(!trace.includes('Telemetry smoke test'));
await until(async () => {
  const r = await fetch(
    prometheus +
      '/api/v1/query?query=' +
      encodeURIComponent('count(up{job=~"journey-api|journey-worker"} == 1)'),
    { signal: AbortSignal.timeout(5000) },
  );
  const body = await r.json();
  return Number(body.data?.result?.[0]?.value?.[1]) === 2;
}, 'Prometheus scrape');
console.log(
  'PASS: API/runtime/provider trace propagated across durable execution; no recipient/body in trace; both metrics targets scrape successfully.',
);
