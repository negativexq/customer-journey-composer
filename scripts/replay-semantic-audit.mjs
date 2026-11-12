import { randomBytes } from 'node:crypto';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import assert from 'node:assert/strict';
// Unpaid replay of recorded model outputs. Deliberately never calls AI or publishes.
const dir = '.tmp/semantic-audit-replay';
await mkdir(dir, { recursive: true });
const input = JSON.parse(await readFile('docs/reports/semantic-live-five-2026-11-12.json', 'utf8'));
const apiOrigin = new URL(process.env.CJC_API_URL || 'http://127.0.0.1:8090');
assert(
  apiOrigin.protocol === 'http:' &&
    ['127.0.0.1', 'localhost', '[::1]'].includes(apiOrigin.hostname),
  'Local API only',
);
const report = {
  source_commit: input.source_commit,
  paid_calls: 0,
  published: false,
  journeys: [],
};
for (const original of input.scenarios) {
  if (!original.workflow || original.id === 'missing-data') continue;
  let cookie = '',
    csrf = '';
  async function call(path, method = 'GET', body) {
    assert(
      !path.includes('/ai/') && !path.endsWith('/publish') && !path.includes('/messages/test-send'),
    );
    const response = await fetch(new URL('/api/v1' + path, apiOrigin), {
      method,
      headers: { 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(15000),
    });
    if (response.headers.has('set-cookie'))
      cookie = response.headers.get('set-cookie').split(';')[0];
    if (!response.ok)
      throw Error(
        method + ' ' + path + ':' + response.status + ' ' + (await response.text()).slice(0, 250),
      );
    return response.json();
  }
  const user = await call('/auth/register', 'POST', {
    name: 'Sandbox audit',
    workspace: 'Semantic replay ' + original.id,
    email: randomBytes(12).toString('hex') + '@example.invalid',
    password: randomBytes(24).toString('hex'),
  });
  csrf = user.csrf_token;
  const j = await call('/journeys', 'POST', { name: original.name + ' — unmodified replay' });
  await call('/journeys/' + j.id + '/draft', 'PUT', {
    name: j.name,
    description: 'Isolated unmodified model output',
    revision: j.revision,
    draft: original.workflow,
  });
  const result = {
    id: original.id,
    journey_id: j.id,
    validation: await call('/journeys/' + j.id + '/validate', 'POST', {}),
    cases: [],
  };
  report.journeys.push(result);
  const base = {
    name: 'Audit',
    start_time: '2026-11-12T09:00:00Z',
    customer: {
      customer_id: 'fictional',
      email: 'alex@example.invalid',
      phone: '+15005550006',
      attributes: {
        first_name: 'Alex',
        product_name: 'Atlas',
        purchased: false,
        activated: false,
        subscribed: false,
        login_count: 1,
        refunded: false,
        reviewed: false,
      },
      consents: { email: true, sms: true },
    },
    entry_event: {
      event_id: 'entry',
      event_name: {
        cart: 'cart_updated',
        onboarding: 'user_registered',
        trial: 'trial_started',
        postpurchase: 'order_delivered',
      }[original.id],
      properties: {
        amount: 89.9,
        amount_display: '€89.90',
        currency: 'EUR',
        order_number: 'DEMO-100',
      },
    },
    frequency_limit: 20,
    frequency_window_seconds: 86400,
  };
  const cases = [];
  function item(name, email, sms, exit, hours) {
    const s = structuredClone(base);
    s.name = name;
    s.assertions = [
      { type: 'enrollment_count', expected: 1 },
      { type: 'no_duplicate_delivery', expected: true },
      { type: 'message_count', channel: 'email', expected: email },
      { type: 'message_count', channel: 'sms', expected: sms },
    ];
    if (exit) s.assertions.push({ type: 'exit_reason', expected: exit });
    const c = { scenario: s, expected_hours: hours };
    cases.push(c);
    return s;
  }
  const event = (s, name, hour) =>
    (s.events = [
      {
        after_seconds: hour * 3600,
        event: { event_id: 'followup', event_name: name, properties: {} },
      },
    ]);
  if (original.id === 'cart') {
    item('No purchase, both channels', 1, 1, 'completed', 73);
    let s = item('Purchase before first wait expires', 0, 0, 'converted', 0.5);
    event(s, 'purchase_completed', 0.5);
    s = item('Purchase after first email', 1, 0, 'converted', 2);
    event(s, 'purchase_completed', 2);
    s = item('No SMS consent', 1, 0, 'completed', undefined);
    s.customer.consents.sms = false;
    s = item('No communication consent', 0, 0, undefined, 0);
    s.customer.consents = { email: false, sms: false };
    cases.at(-1).must_not_convert = true;
    s = item('Cart below entry threshold', 0, 0, undefined, undefined);
    s.entry_event.properties.amount = 49;
    s.assertions = s.assertions.map((a) =>
      a.type === 'enrollment_count' ? { ...a, expected: 0 } : a,
    );
    s = item('Profile purchase changes before email', 0, 0, 'converted', 1);
    s.attribute_changes = [{ after_seconds: 1800, attributes: { purchased: true } }];
    s = item('Profile purchase changes before SMS', 1, 0, 'converted', 25);
    s.attribute_changes = [{ after_seconds: 24 * 3600, attributes: { purchased: true } }];
    s = item('Fractional amount still below fifty', 0, 0, undefined, undefined);
    s.entry_event.properties.amount = 49.995;
    s.assertions = s.assertions.map((a) =>
      a.type === 'enrollment_count' ? { ...a, expected: 0 } : a,
    );
    item('Exactly fifty enters', 1, 1, 'completed', 73).entry_event.properties.amount = 50;
  } else if (original.id === 'onboarding') {
    item('Inactive for three days', 1, 1, 'completed', 72);
    let s = item('Already active at entry', 0, 0, 'converted', 0);
    s.customer.attributes.activated = true;
    s = item('Activation after 30 minutes', 1, 0, 'converted', 0.5);
    event(s, 'account_activated', 0.5);
    s = item('Activation only in profile at hour 47', 1, 0, 'converted', 48);
    s.attribute_changes = [{ after_seconds: 47 * 3600, attributes: { activated: true } }];
    s = item('No SMS consent, final day retained', 1, 0, 'completed', 72);
    s.customer.consents.sms = false;
    s = item('No communication consent', 0, 0, undefined, 0);
    s.customer.consents = { email: false, sms: false };
    cases.at(-1).must_not_convert = true;
  } else if (original.id === 'trial') {
    item('Low engagement, no subscription', 2, 1, 'completed', 168);
    let s = item('Three logins, skip getting-started email', 1, 1, 'completed', 168);
    s.customer.attributes.login_count = 3;
    s = item('Already subscribed at entry', 0, 0, 'converted', 0);
    s.customer.attributes.subscribed = true;
    s = item('Subscription on day two', 1, 0, 'converted', 48);
    event(s, 'subscription_started', 48);
    s = item('Profile subscription at hour 71', 1, 0, 'converted', 72);
    s.attribute_changes = [{ after_seconds: 71 * 3600, attributes: { subscribed: true } }];
    s = item('Profile subscription at hour 119', 2, 0, 'converted', 120);
    s.attribute_changes = [{ after_seconds: 119 * 3600, attributes: { subscribed: true } }];
    s = item('Missing personalization values use fallbacks', 2, 1, 'completed', 168);
    delete s.customer.attributes.first_name;
    delete s.customer.attributes.product_name;
  } else {
    item('No refund or review', 1, 1, 'completed', 144);
    let s = item('Review at hour 96', 1, 0, 'converted', 96);
    event(s, 'review_submitted', 96);
    s = item('Refund before email must not convert', 0, 0, 'completed', 24);
    event(s, 'order_refunded', 24);
    cases.at(-1).must_not_convert = true;
    s = item('Refund after email must stop SMS without conversion', 1, 0, 'completed', 100);
    event(s, 'order_refunded', 100);
    cases.at(-1).must_not_convert = true;
    s = item('Already refunded profile excluded at day-three check', 0, 0, undefined, 72);
    s.customer.attributes.refunded = true;
    cases.at(-1).must_not_convert = true;
    s = item('No email consent, continue with permitted SMS', 0, 1, 'completed', 144);
    s.customer.consents.email = false;
  }
  for (const c of cases) {
    try {
      const run = await call('/test-runs', 'POST', { journey_id: j.id, scenario: c.scenario });
      const done = await call('/test-runs/' + run.id + '/advance', 'POST', {
        operation: 'complete',
        revision: run.revision,
      });
      const hours = (Date.parse(done.state.now) - Date.parse(c.scenario.start_time)) / 3600000;
      const inbox = await call('/test-inbox?test_run_id=' + run.id);
      const messages = [];
      for (const row of inbox) {
        const detail = await call('/test-messages/' + row.id);
        messages.push({
          channel: row.channel,
          status: row.status,
          mode: row.mode,
          node: row.node_label,
          message: detail.message,
          decision: detail.decision,
        });
        assert.equal(row.mode, 'SIMULATED');
      }
      const timing = c.expected_hours === undefined || hours === c.expected_hours;
      const nonConversion = !c.must_not_convert || done.state.exit_reason !== 'converted';
      result.cases.push({
        name: c.scenario.name,
        scenario: c.scenario,
        pass: done.state.result === 'PASS' && timing && nonConversion,
        engine_result: done.state.result,
        actual_hours: hours,
        expected_hours: c.expected_hours,
        exit_reason: done.state.exit_reason,
        not_converted: nonConversion,
        assertions: done.state.assertions,
        messages,
        trace: done.state.trace,
        steps: done.state.steps,
      });
    } catch (e) {
      result.cases.push({ name: c.scenario.name, pass: false, error: e.message });
    }
    console.log(original.id, result.cases.at(-1).name, result.cases.at(-1).pass ? 'PASS' : 'FAIL');
    await writeFile(dir + '/replay.json', JSON.stringify(report, null, 2) + '\n');
  }
  result.isolation = {
    no_customers: (await call('/customers')).length === 0,
    no_deliveries: (await call('/deliveries')).length === 0,
    no_executions: (await call('/journeys/' + j.id + '/executions')).length === 0,
    no_versions: (await call('/journeys/' + j.id + '/versions')).length === 0,
  };
}
report.passed = report.journeys.flatMap((j) => j.cases).filter((c) => c.pass).length;
report.total = report.journeys.flatMap((j) => j.cases).length;
await writeFile(dir + '/replay.json', JSON.stringify(report, null, 2) + '\n');
console.log('TOTAL', report.passed + '/' + report.total);

process.exitCode = report.passed === report.total ? 0 : 1;
