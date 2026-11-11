import { mkdir, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

export function localDemoURL(value = 'http://127.0.0.1:8090') {
  const url = new URL(value);
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    url.pathname !== '/'
  )
    throw new Error('Demo seeding requires a loopback development API origin.');
  return url.origin;
}
export async function seedProductDemo({ base = process.env.CJC_API_URL, log = console.log } = {}) {
  base = localDemoURL(base);
  let cookie = '',
    csrf = '';
  async function call(path, method = 'GET', body) {
    for (let attempt = 0; attempt < 12; attempt++) {
      const response = await fetch(base + '/api/v1' + path, {
        method,
        headers: {
          Accept: 'application/json',
          ...(cookie ? { Cookie: cookie } : {}),
          ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        },
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: AbortSignal.timeout(25000),
      });
      if (response.status === 429) {
        await new Promise((resolve) => setTimeout(resolve, 500));
        continue;
      }
      if (response.status === 404 && method === 'GET') return null;
      if (!response.ok) throw new Error(`${method} ${path}: HTTP ${response.status}`);
      if (response.headers.has('set-cookie'))
        cookie = response.headers.get('set-cookie').split(';')[0];
      return response.json();
    }
    throw new Error('Demo API remained rate limited.');
  }
  const demo = await call('/auth/demo');
  if (!demo?.enabled || !demo.email || !demo.password)
    throw new Error('The development-only demo account must be enabled.');
  const identity = await call('/auth/login', 'POST', {
    email: demo.email,
    password: demo.password,
  });
  csrf = identity.csrf_token;
  let journeys = await call('/journeys?limit=200');
  const tagged = (key) => journeys.find((j) => j.tags.includes('product-demo:' + key));
  async function lifecycle(journey, action) {
    const current = await call('/journeys/' + journey.id);
    return call(`/journeys/${journey.id}/${action}`, 'POST', { revision: current.revision });
  }
  const recipe = await call('/templates/abandoned-cart-eur');
  let canonical = journeys.find((j) => j.tags.includes('eur-recovery'));
  if (!canonical) {
    canonical = await call('/templates/abandoned-cart-eur/create-journey', 'POST', {});
    journeys.push(canonical);
  }
  async function completedRun(journey, scenario) {
    const run = await call('/test-runs', 'POST', { journey_id: journey.id, scenario });
    const done = await call(`/test-runs/${run.id}/advance`, 'POST', {
      operation: 'complete',
      revision: run.revision,
    });
    if (done.state.result !== 'PASS')
      throw new Error(`Demo scenario did not pass: ${scenario.name}`);
    return done;
  }
  if (!tagged('ready')) {
    const scenarios = await call('/test-scenarios');
    for (const scenario of recipe.scenarios) {
      if (!scenarios.some((s) => s.journey_id === canonical.id && s.name === scenario.name))
        await call('/test-scenarios', 'POST', { journey_id: canonical.id, scenario });
      await completedRun(canonical, scenario);
    }
    log('Prepared five recovery scenarios and their isolated inbox outputs.');
  }
  const node = (id, type, label, x, y, config) => ({ id, type, label, position: { x, y }, config });
  const edge = (id, source, target, port) => ({ id, source, target, port });
  const cart = {
    schema_version: 1,
    reentry: { mode: 'cooldown', cooldown_seconds: 604800 },
    global_exit: { event_name: 'demo_purchase_completed' },
    nodes: [
      node('entry', 'event_trigger', 'Cart abandoned', 240, 0, {
        event_name: 'demo_cart_abandoned',
      }),
      node('consent', 'condition', 'SMS permission?', 240, 170, {
        rule: { path: 'customer.consents.sms', operator: 'equals', value: true },
      }),
      node('wait', 'wait', 'Wait 3 seconds · demo', 240, 340, {
        duration_seconds: 3,
        timezone: 'UTC',
      }),
      node('reminder', 'sms', 'Your cart is waiting', 240, 510, {
        text: 'Hi {{ customer.first_name }}! Your €{{ event.properties.amount_display }} cart is waiting. Return to your cart: https://example.com/cart',
      }),
      node('purchase', 'wait_event', 'Wait for purchase · demo', 240, 680, {
        event_name: 'demo_purchase_completed',
        timeout_seconds: 15,
      }),
      node('offer', 'sms', '10% off · COMEBACK10', 500, 850, {
        text: '{{ customer.first_name }}, use COMEBACK10 for 10% off your €{{ event.properties.amount_display }} cart. https://example.com/cart',
      }),
      node('converted', 'exit', 'Converted', 0, 850, { reason: 'converted' }),
      node('done', 'exit', 'Recovery completed', 500, 1020, { reason: 'completed' }),
      node('no_consent', 'exit', 'No channel permission', 620, 340, {
        reason: 'condition_not_met',
      }),
    ],
    edges: [
      edge('a', 'entry', 'consent', 'started'),
      edge('b', 'consent', 'wait', 'true'),
      edge('c', 'consent', 'no_consent', 'false'),
      edge('d', 'wait', 'reminder', 'completed'),
      edge('e', 'reminder', 'purchase', 'next'),
      edge('f', 'purchase', 'converted', 'matched'),
      edge('g', 'purchase', 'offer', 'timeout'),
      edge('h', 'offer', 'done', 'next'),
    ],
  };
  const welcome = {
    schema_version: 1,
    reentry: { mode: 'once' },
    global_exit: { event_name: 'demo_activation_completed' },
    nodes: [
      node('entry', 'event_trigger', 'New account', 240, 0, { event_name: 'demo_user_registered' }),
      node('wait', 'wait', 'Wait 3 seconds · demo', 240, 170, {
        duration_seconds: 3,
        timezone: 'UTC',
      }),
      node('welcome', 'sms', 'Welcome aboard', 240, 340, {
        text: 'Welcome, {{ customer.first_name }}! Your workspace is ready. Get started: https://example.com/start',
      }),
      node('activation', 'wait_event', 'Wait for activation', 240, 510, {
        event_name: 'demo_activation_completed',
        timeout_seconds: 172800,
      }),
      node('converted', 'exit', 'Activated', 0, 680, { reason: 'converted' }),
      node('done', 'exit', 'Onboarding completed', 480, 680, { reason: 'completed' }),
    ],
    edges: [
      edge('a', 'entry', 'wait', 'started'),
      edge('b', 'wait', 'welcome', 'completed'),
      edge('c', 'welcome', 'activation', 'next'),
      edge('d', 'activation', 'converted', 'matched'),
      edge('e', 'activation', 'done', 'timeout'),
    ],
  };
  const longWait = (event, seconds) => ({
    schema_version: 1,
    reentry: { mode: 'after_completion' },
    nodes: [
      node('entry', 'event_trigger', 'Customer eligible', 240, 0, { event_name: event }),
      node('wait', 'wait', 'Wait before follow-up', 240, 170, {
        duration_seconds: seconds,
        timezone: 'UTC',
      }),
      node('message', 'sms', 'A personal check-in', 240, 340, {
        text: 'Hi {{ customer.first_name }}, we would love to see you again. https://example.com',
      }),
      node('done', 'exit', 'Completed', 240, 510, { reason: 'completed' }),
    ],
    edges: [
      edge('a', 'entry', 'wait', 'started'),
      edge('b', 'wait', 'message', 'completed'),
      edge('c', 'message', 'done', 'next'),
    ],
  });
  async function ensureJourney(key, name, workflow, description) {
    let j = tagged(key);
    if (!j) {
      j = await call('/journeys', 'POST', {
        name,
        description,
        tags: ['demo', 'product-demo:' + key],
      });
      j = await call(`/journeys/${j.id}/draft`, 'PUT', {
        name,
        description,
        tags: j.tags,
        draft: workflow,
        revision: j.revision,
      });
      journeys.push(j);
    } else j = await call('/journeys/' + j.id);
    if (j.status === 'Draft' && key !== 'ready') {
      await completedRun(j, {
        name: name + ' · publish check',
        customer: {
          customer_id: 'demo_gate',
          first_name: 'Alex',
          attributes: { first_name: 'Alex' },
          phone: '+12025550100',
          consents: { sms: true },
        },
        entry_event: {
          event_name: workflow.nodes[0].config.event_name,
          properties: { amount_display: '89.90' },
        },
        assertions: [
          { type: 'state', expected: 'COMPLETED' },
          { type: 'no_duplicate_delivery', expected: true },
        ],
      });
      await call(`/journeys/${j.id}/publish`, 'POST', { revision: j.revision });
      j = await call('/journeys/' + j.id);
    }
    return j;
  }
  const cartJourney = await ensureJourney(
    'cart',
    'Cart Recovery · live demo',
    cart,
    'Synthetic customers, accelerated real-time waits and the mock SMS provider. No real customer messages.',
  );
  const welcomeJourney = await ensureJourney(
    'welcome',
    'Welcome & Activation',
    welcome,
    'Demo onboarding with permission checks, activation exits and durable event waiting. SMS is mocked.',
  );
  const winback = await ensureJourney(
    'winback',
    'Win-back Customers',
    longWait('demo_customer_inactive', 604800),
    'Demo re-engagement journey. Paused after enrollment to demonstrate lifecycle controls. SMS is mocked.',
  );
  const followup = await ensureJourney(
    'followup',
    'Post-purchase Check-in',
    longWait('demo_post_purchase', 43200),
    'Demo follow-up in sunset mode: existing participants continue, new entry is closed. SMS is mocked.',
  );
  if (!tagged('ready')) {
    for (const j of [cartJourney, welcomeJourney, winback, followup])
      if (j.status === 'Paused') await lifecycle(j, 'resume');
    const names = [
      'Alex Morgan',
      'Maya Patel',
      'Leo Martin',
      'Emma Wilson',
      'Lucas Silva',
      'Sofia Garcia',
      'Noah Kim',
      'Amelia Brown',
      'Ethan Davis',
      'Isabella Rossi',
      'Liam Johnson',
      'Olivia Chen',
      'Oliver Taylor',
      'Ava Thompson',
      'Elijah Anderson',
      'Mia Robinson',
      'James Lewis',
      'Charlotte Walker',
    ];
    for (let i = 0; i < 54; i++) {
      const id = 'showcase_customer_' + String(i + 1).padStart(3, '0');
      const [first, last] = names[i % names.length].split(' ');
      if (!(await call('/customers/' + id)))
        await call('/customers', 'POST', {
          customer_id: id,
          external_id: 'SHOP-' + (1000 + i),
          email: `${first.toLowerCase()}.${last.toLowerCase()}.${i + 1}@example.invalid`,
          phone: '+1202555' + String(100 + i).padStart(4, '0'),
          locale: 'en',
          timezone: ['Europe/Paris', 'Europe/London', 'America/New_York'][i % 3],
          attributes: {
            first_name: first,
            last_name: last,
            cart_purchased: false,
            activated: false,
            total_orders: i % 5,
            plan: i % 4 === 0 ? 'Premium' : 'Standard',
            demo: true,
          },
          consents: { email: true, sms: i < 36 ? i % 6 !== 5 : i % 6 !== 5 },
        });
    }
    async function event(id, customer, name, properties = {}) {
      if (await call('/events/' + id)) return null;
      return call('/events', 'POST', {
        event_id: id,
        customer_id: customer,
        event_name: name,
        occurred_at: new Date().toISOString(),
        properties,
      });
    }
    const customers = Array.from(
      { length: 54 },
      (_, i) => 'showcase_customer_' + String(i + 1).padStart(3, '0'),
    );
    for (let i = 0; i < 36; i++) {
      await event('showcase_cart_' + i, customers[i], 'demo_cart_abandoned', {
        cart_id: 'CART-' + (2000 + i),
        amount: [89.9, 149, 245, 65][i % 4],
        amount_display: [89.9, 149, 245, 65][i % 4].toFixed(2),
        currency: 'EUR',
      });
      if (i < 6)
        await event('showcase_purchase_early_' + i, customers[i], 'demo_purchase_completed', {
          cart_id: 'CART-' + (2000 + i),
          amount: 89.9,
          currency: 'EUR',
        });
    }
    for (let i = 6; i < 12; i++) {
      if (!(await call('/events/showcase_purchase_after_' + i))) {
        const deadline = Date.now() + 12000;
        while (i % 6 !== 5 && Date.now() < deadline) {
          const sent = await call('/deliveries?customer_id=' + customers[i]);
          if (sent.some((d) => d.status === 'ACCEPTED')) break;
          await new Promise((resolve) => setTimeout(resolve, 250));
        }
        await event('showcase_purchase_after_' + i, customers[i], 'demo_purchase_completed', {
          cart_id: 'CART-' + (2000 + i),
          amount: 149,
          currency: 'EUR',
        });
      }
    }
    for (let i = 36; i < 54; i++) {
      await event('showcase_signup_' + i, customers[i], 'demo_user_registered');
      if (i < 40)
        await event('showcase_activation_' + i, customers[i], 'demo_activation_completed');
    }
    for (let i = 0; i < 12; i++)
      await event('showcase_inactive_' + i, customers[i], 'demo_customer_inactive');
    for (let i = 24; i < 30; i++)
      await event('showcase_followup_' + i, customers[i], 'demo_post_purchase');
    await lifecycle(winback, 'pause');
    await lifecycle(followup, 'sunset');
    const end = Date.now() + 60000;
    while (Date.now() < end) {
      const rows = await call(`/journeys/${cartJourney.id}/executions?limit=100`);
      if (rows.length === 36 && rows.every((e) => !['RUNNING', 'WAITING'].includes(e.state))) break;
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
    const rows = await call(`/journeys/${cartJourney.id}/executions?limit=100`);
    if (rows.length !== 36 || rows.some((e) => ['RUNNING', 'WAITING'].includes(e.state)))
      throw new Error('Demo cart executions did not settle. Retry seeding to resume.');
    await ensureJourney(
      'ready',
      'VIP Loyalty · next campaign',
      longWait('demo_vip_eligible', 86400),
      'Draft demo campaign, ready to personalize and test.',
    );
    log(
      'Prepared 54 fictional profiles, 72 live demo enrollments and isolated recovery test runs.',
    );
  }
  const runs = await call(`/test-runs?journey_id=${canonical.id}&limit=200`);
  const inboxRun = runs.find((r) => r.result === 'PASS' && r.message_count === 2);
  const tracked = await call(
    `/journeys/${welcomeJourney.id}/executions?customer_id=showcase_customer_041&limit=1`,
  );
  const profileRows = await call('/customers?limit=200');
  const executionRows = await Promise.all(
    [cartJourney, welcomeJourney, winback, followup].map((j) =>
      call(`/journeys/${j.id}/executions?limit=100`),
    ),
  );
  const metadata = {
    canonicalJourneyId: canonical.id,
    inboxRunId: inboxRun?.id,
    cartJourneyId: cartJourney.id,
    welcomeJourneyId: welcomeJourney.id,
    monitorExecutionId: tracked[0]?.id,
    customerId: 'showcase_customer_007',
    synthetic: true,
    customerCount: profileRows.filter((c) => c.customer_id.startsWith('showcase_customer_')).length,
    executionCount: executionRows.reduce((total, rows) => total + rows.length, 0),
  };
  await mkdir('.tmp', { recursive: true });
  await writeFile('.tmp/product-demo.json', JSON.stringify(metadata, null, 2) + '\n');
  log('Demo ready. Emails remain in Simulation; live showcase sends use mock SMS only.');
  return metadata;
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    await seedProductDemo();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
