// Replays a recorded live proposal through Test Lab. Never calls an LLM or publishes.
import assert from 'node:assert/strict';
import { createHash, randomBytes } from 'node:crypto';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { dirname } from 'node:path';
const base = new URL(process.env.CJC_API_URL || 'http://127.0.0.1:8090');
assert(
  base.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(base.hostname),
  'This verification is local only.',
);
const proposal = JSON.parse(
  await readFile(
    new URL('../tests/fixtures/ai/luna-recovery-2026-11-11.json', import.meta.url),
    'utf8',
  ),
);
// PostgreSQL JSONB may reorder object keys; compare semantic content, preserving array order.
const canonical = (value) =>
  Array.isArray(value)
    ? value.map(canonical)
    : value !== null && typeof value === 'object'
      ? Object.fromEntries(
          Object.keys(value)
            .sort()
            .map((key) => [key, canonical(value[key])]),
        )
      : value;
const hash = (value) =>
  createHash('sha256')
    .update(JSON.stringify(canonical(value)))
    .digest('hex');
const originalHash = hash(proposal.workflow);
let cookie = '',
  csrf = '';
async function call(path, method = 'GET', data) {
  assert(
    !path.includes('/ai/') && !path.includes('/publish') && !path.includes('/messages/test-send'),
    'Generation, publication and provider sends are forbidden in this replay.',
  );
  const response = await fetch(new URL('/api/v1' + path, base), {
    method,
    headers: { 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf },
    body: data === undefined ? undefined : JSON.stringify(data),
    signal: AbortSignal.timeout(15000),
  });
  if (!response.ok) throw new Error(`${method} ${path}: HTTP ${response.status}`);
  if (response.headers.has('set-cookie')) cookie = response.headers.get('set-cookie').split(';')[0];
  return response.json();
}
const user = await call('/auth/register', 'POST', {
  name: 'Scenario verification',
  workspace: 'Isolated ten-scenario recovery verification',
  email: randomBytes(12).toString('hex') + '@example.invalid',
  password: randomBytes(24).toString('hex'),
});
csrf = user.csrf_token;
const recipe = await call('/templates/abandoned-cart-eur');
const clone = (value) => structuredClone(value);
const cases = recipe.scenarios.map((scenario, index) => ({
  id: index + 1,
  scenario: clone(scenario),
}));
const custom = (id, name) => {
  const item = { id, scenario: clone(recipe.scenarios[2]) };
  item.scenario.name = name;
  return item;
};
const baseAssertions = () => [{ type: 'no_duplicate_delivery', expected: true }];
const below = custom(6, 'F · Below threshold: EUR 49.99');
below.scenario.entry_event.properties.amount = 49.99;
below.scenario.entry_event.properties.amount_display = '49.99';
below.scenario.assertions = [
  ...baseAssertions(),
  { type: 'enrollment_count', expected: 0 },
  { type: 'no_message' },
];
cases.push(below);
const boundary = custom(7, 'G · Exactly EUR 50 qualifies');
boundary.scenario.entry_event.properties.amount = 50;
boundary.scenario.entry_event.properties.amount_display = '50.00';
cases.push(boundary);
const noConsent = custom(8, 'H · Neither channel consent: immediate exit');
noConsent.scenario.customer.consents = { email: false, sms: false };
noConsent.scenario.assertions = [
  ...baseAssertions(),
  { type: 'no_message' },
  { type: 'exit_reason', expected: 'condition_not_met' },
];
noConsent.expected_elapsed_seconds = 0;
cases.push(noConsent);
const already = custom(9, 'I · Cart already purchased in profile');
already.scenario.customer.attributes.cart_purchased = true;
already.scenario.assertions = [
  ...baseAssertions(),
  { type: 'no_message' },
  { type: 'exit_reason', expected: 'converted' },
];
cases.push(already);
const final = custom(10, 'J · Profile purchase update at +72h, without purchase event');
final.scenario.attribute_changes = [
  { after_seconds: 72 * 3600, attributes: { cart_purchased: true } },
];
final.scenario.assertions = [
  ...baseAssertions(),
  { type: 'message_count', channel: 'email', expected: 1 },
  { type: 'message_count', channel: 'sms', expected: 1 },
  { type: 'exit_reason', expected: 'converted' },
];
cases.push(final);
assert.equal(cases.length, 10);
const generated = await call('/journeys', 'POST', {
  name: 'Recorded Luna recovery · 10 scenarios',
});
await call(`/journeys/${generated.id}/draft`, 'PUT', {
  name: generated.name,
  description: proposal.description,
  draft: proposal.workflow,
  revision: generated.revision,
});
const reference = await call('/templates/abandoned-cart-eur/create-journey', 'POST', {
  name: 'Canonical reference · same 10 scenarios',
});
const initialMetrics = (await call('/overview')).metrics;
const report = {
  recorded_at: new Date().toISOString(),
  model_fixture: 'gpt-6-luna',
  new_generation_calls: 0,
  workflow_sha256: originalHash,
  test_contract:
    'Original workflow is unchanged; canonical reference runs identical business assertions.',
  cases: [],
  isolation: {},
};
async function runCase(journey, item) {
  const scenario = clone(item.scenario);
  const run = await call('/test-runs', 'POST', { journey_id: journey.id, scenario });
  const done = await call(`/test-runs/${run.id}/advance`, 'POST', {
    operation: 'complete',
    revision: run.revision,
  });
  const state = done.state;
  const elapsed = (Date.parse(state.now) - Date.parse(scenario.start_time)) / 1000;
  const extra =
    item.expected_elapsed_seconds === undefined
      ? []
      : [
          {
            description: 'elapsed_seconds',
            pass: elapsed === item.expected_elapsed_seconds,
            expected: item.expected_elapsed_seconds,
            actual: elapsed,
          },
        ];
  const assertions = state.assertions.map((a) => ({
    type: a.assertion.type,
    channel: a.assertion.channel,
    expected: a.assertion.expected,
    actual: a.actual,
    pass: a.pass,
  }));
  assertions.push(...extra);
  const inbox = await call(`/test-inbox?test_run_id=${run.id}`);
  assert(
    inbox.every((m) => m.mode === 'SIMULATED' && m.test_run_id === run.id),
    'Inbox leaked another run or a real send.',
  );
  return {
    result: state.result === 'PASS' && assertions.every((a) => a.pass) ? 'PASS' : 'FAIL',
    state: state.status,
    exit_reason: state.exit_reason,
    elapsed_seconds: elapsed,
    enrollment_count: state.enrollment_count,
    messages: inbox.map((m) => ({ channel: m.channel, status: m.status })),
    assertions,
  };
}
for (const item of cases) {
  const control = await runCase(reference, item);
  const actual = await runCase(generated, item);
  report.cases.push({ id: item.id, name: item.scenario.name, control, generated: actual });
  console.log(
    `${item.id}. ${item.scenario.name}: generated=${actual.result}; reference=${control.result}`,
  );
}
for (const journey of [generated, reference]) {
  const saved = await call(`/journeys/${journey.id}`);
  assert.equal(saved.status, 'Draft');
  assert.equal((await call(`/journeys/${journey.id}/versions`)).length, 0);
  assert.equal((await call(`/journeys/${journey.id}/executions`)).length, 0);
  if (journey.id === generated.id)
    assert.equal(hash(saved.draft), originalHash, 'Generated workflow changed during replay.');
}
assert.equal((await call('/customers')).length, 0);
assert.equal((await call('/deliveries')).length, 0);
assert.deepEqual((await call('/overview')).metrics, initialMetrics);
assert.equal(hash(proposal.workflow), originalHash);
report.isolation = {
  both_drafts: true,
  no_versions: true,
  no_live_customers: true,
  no_live_executions: true,
  no_real_delivery_jobs: true,
  no_production_metrics: true,
  workflow_unchanged: true,
};
report.summary = {
  generated_pass: report.cases.filter((c) => c.generated.result === 'PASS').length,
  reference_pass: report.cases.filter((c) => c.control.result === 'PASS').length,
  total: cases.length,
};
const out = process.env.CJC_SCENARIO_REPORT || '.tmp/luna-ten-scenarios.json';
await mkdir(dirname(out), { recursive: true });
await writeFile(out, JSON.stringify(report, null, 2) + '\n');
console.log(
  `Completed: generated ${report.summary.generated_pass}/10; reference ${report.summary.reference_pass}/10. Report: ${out}`,
);
// Acceptance failures stay failures: this command deliberately exits nonzero until the candidate passes.
if (report.summary.generated_pass !== 10 || report.summary.reference_pass !== 10)
  process.exitCode = 1;
