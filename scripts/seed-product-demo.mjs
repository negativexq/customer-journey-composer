import { mkdir, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import {
  showcaseRecipe,
  showcaseName,
  showcaseDescription,
  showcaseTag,
} from './showcase-recipe.mjs';

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
function localSQL(sql) {
  return execFileSync(
    'docker',
    [
      'compose',
      'exec',
      '-T',
      'postgres',
      'sh',
      '-c',
      'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At',
    ],
    { input: sql, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] },
  );
}
export async function seedProductDemo({
  base = process.env.CJC_API_URL,
  log = console.log,
  reset = false,
} = {}) {
  base = localDemoURL(base);
  let cookie = '',
    csrf = '';
  async function call(path, method = 'GET', body) {
    const response = await fetch(base + '/api/v1' + path, {
      method,
      headers: { Cookie: cookie, 'X-CSRF-Token': csrf, 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(25000),
    });
    if (response.status === 404 && method === 'GET') return null;
    if (!response.ok) throw new Error(`${method} ${path}: HTTP ${response.status}`);
    if (response.headers.has('set-cookie'))
      cookie = response.headers.get('set-cookie').split(';')[0];
    return response.json();
  }
  const config = await call('/auth/demo');
  if (!config?.enabled) throw new Error('Development demo must be enabled.');
  async function login() {
    const identity = await call('/auth/login', 'POST', {
      email: config.email,
      password: config.password,
    });
    csrf = identity.csrf_token;
    return identity;
  }
  let identity = await login();
  let journeys = await call('/journeys?limit=200');
  if (reset) {
    if (![identity.id, identity.workspace_id].every((v) => /^[0-9a-f-]{36}$/.test(v)))
      throw new Error('Invalid demo identity.');
    for (let journey of journeys) {
      if (['Active', 'Sunset'].includes(journey.status))
        journey = await call(`/journeys/${journey.id}/pause`, 'POST', {
          revision: journey.revision,
        });
      const executions = await call(`/journeys/${journey.id}/executions?limit=200`);
      for (const execution of executions.filter((e) =>
        ['RUNNING', 'WAITING', 'FAILED'].includes(e.state),
      ))
        await call(`/executions/${execution.id}/cancel`, 'POST', {});
      if (journey.status === 'Paused')
        journey = await call(`/journeys/${journey.id}/complete`, 'POST', {
          revision: journey.revision,
        });
      if (['Draft', 'Completed'].includes(journey.status))
        await call(`/journeys/${journey.id}/archive`, 'POST', { revision: journey.revision });
    }
    const result = localSQL(
      `BEGIN; SELECT pg_advisory_xact_lock(716294804); DO $$ DECLARE prior users%ROWTYPE; fresh uuid; owner uuid; BEGIN SELECT u.* INTO STRICT prior FROM users u JOIN workspaces w ON w.id=u.workspace_id WHERE u.id='${identity.id}' AND w.id='${identity.workspace_id}' AND w.slug='local-demo' FOR UPDATE OF u,w; UPDATE workspaces SET slug='archived-demo-'||id::text,name='Archived demo workspace' WHERE id=prior.workspace_id; UPDATE users SET email='archived-demo-'||id::text||'@example.invalid' WHERE id=prior.id; DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE workspace_id=prior.workspace_id); UPDATE api_keys SET revoked_at=now() WHERE workspace_id=prior.workspace_id AND revoked_at IS NULL; INSERT INTO workspaces(name,slug) VALUES('Northstar Commerce','local-demo') RETURNING id INTO fresh; INSERT INTO users(workspace_id,email,name,password_hash,role) VALUES(fresh,prior.email,'Alex Morgan',prior.password_hash,'Admin') RETURNING id INTO owner; INSERT INTO demo_seed_state(workspace_id,version) VALUES(fresh,1); INSERT INTO workspace_settings(workspace_id) VALUES(fresh); INSERT INTO audit_logs(workspace_id,actor_id,action,resource_id,details) VALUES(fresh,owner,'demo.workspace_reset',fresh,jsonb_build_object('previous_workspace',prior.workspace_id,'historical_records_retained',true)); END $$; COMMIT;`,
    );
    if (!result.includes('COMMIT')) throw new Error('Demo reset did not commit.');
    await mkdir('.tmp/clean-showcase', { recursive: true });
    await writeFile(
      '.tmp/clean-showcase/reset-journal.json',
      JSON.stringify(
        {
          previous_workspace_id: identity.workspace_id,
          previous_journey_count: journeys.length,
          historical_records_retained: true,
          reset_at: new Date().toISOString(),
        },
        null,
        2,
      ) + '\n',
    );
    cookie = '';
    csrf = '';
    identity = await login();
    journeys = [];
    log(
      'Reset demo access to a clean Northstar workspace; previous records retained in the archived workspace.',
    );
  }
  const recipe = showcaseRecipe();
  let journey = journeys.find((j) => j.tags.includes(showcaseTag));
  async function run(j, scenario, save = false) {
    const saved = save
      ? await call('/test-scenarios', 'POST', { journey_id: j.id, scenario })
      : null;
    const created = await call('/test-runs', 'POST', {
      journey_id: j.id,
      ...(saved ? { scenario_id: saved.id, use_current_draft: true } : { scenario }),
    });
    const done = await call(`/test-runs/${created.id}/advance`, 'POST', {
      operation: 'complete',
      revision: created.revision,
    });
    if (done.state.result !== 'PASS') throw new Error(`Showcase test failed: ${scenario.name}`);
    return done;
  }
  if (!journey) {
    execFileSync(
      'docker',
      [
        'compose',
        'exec',
        '-T',
        'worker',
        'sh',
        '-c',
        'test "$SMTP_HOST" = mailpit && test "$SMTP_PORT" = 1025',
      ],
      { stdio: 'pipe' },
    );
    await call('/data-catalog', 'PUT', { entries: recipe.catalog });
    const live = showcaseRecipe({ accelerated: true });
    journey = await call('/journeys', 'POST', {
      name: showcaseName,
      description: showcaseDescription,
      tags: [showcaseTag, 'Retention', 'Email + SMS'],
    });
    journey = await call(`/journeys/${journey.id}/draft`, 'PUT', {
      name: journey.name,
      description: journey.description,
      tags: journey.tags,
      draft: live.workflow,
      revision: journey.revision,
    });
    for (const scenario of live.scenarios) await run(journey, scenario);
    await call(`/journeys/${journey.id}/publish`, 'POST', { revision: journey.revision });
    journey = await call('/journeys/' + journey.id);
    journey = await call(`/journeys/${journey.id}/draft`, 'PUT', {
      name: journey.name,
      description: journey.description,
      tags: journey.tags,
      draft: recipe.workflow,
      revision: journey.revision,
    });
    for (const scenario of recipe.scenarios) await run(journey, scenario, true);
    const names = [
      'Alex Morgan',
      'Sophie Chen',
      'Noah Williams',
      'Emma Wilson',
      'Lucas Silva',
      'Amelia Brown',
      'Ethan Davis',
      'Isabella Rossi',
      'Liam Johnson',
      'Olivia Taylor',
      'Ava Thompson',
      'James Lewis',
    ];
    for (const [i, name] of names.entries()) {
      const [first, last] = name.split(' ');
      const id = name.toLowerCase().replaceAll(' ', '-');
      await call('/customers', 'POST', {
        customer_id: id,
        external_id: 'NS-' + (1001 + i),
        email: `${first.toLowerCase()}.${last.toLowerCase()}@example.invalid`,
        phone: '+1202555' + String(100 + i).padStart(4, '0'),
        locale: 'en',
        timezone: 'Europe/London',
        attributes: {
          first_name: first,
          last_name: last,
          loyalty_tier: i % 3 === 0 ? 'Gold' : 'Member',
          total_orders: i % 5,
          cart_total: 89.9,
        },
        consents: { email: i !== 10, sms: i !== 11 },
      });
      await call('/events', 'POST', {
        event_id: 'northstar-cart-' + id,
        customer_id: id,
        event_name: 'cart_updated',
        occurred_at: new Date().toISOString(),
        properties: {
          cart_id: 'CART-' + id,
          amount: 89.9,
          amount_display: '€89.90',
          currency: 'EUR',
        },
      });
      if (i === 1 || i === 2)
        await call('/events', 'POST', {
          event_id: 'northstar-early-' + id,
          customer_id: id,
          event_name: 'purchase_completed',
          occurred_at: new Date().toISOString(),
          properties: { cart_id: 'CART-' + id },
        });
    }
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      const rows = await call(`/journeys/${journey.id}/executions?limit=100`);
      if (
        rows.filter((e) => e.state === 'WAITING' && e.current_node_id === 'purchase_wait').length >=
        9
      )
        break;
      await new Promise((r) => setTimeout(r, 500));
    }
    for (const id of ['emma-wilson', 'lucas-silva', 'amelia-brown'])
      await call('/events', 'POST', {
        event_id: 'northstar-returned-' + id,
        customer_id: id,
        event_name: 'purchase_completed',
        occurred_at: new Date().toISOString(),
        properties: { cart_id: 'CART-' + id },
      });
  }
  if (!(await call('/customers/maya-patel'))) {
    const end = Date.now() + 60000;
    while (Date.now() < end) {
      const rows = await call(`/journeys/${journey.id}/executions?limit=100`);
      if (rows.length >= 12 && rows.every((e) => !['RUNNING', 'WAITING'].includes(e.state))) break;
      await new Promise((r) => setTimeout(r, 500));
    }
    const settled = await call(`/journeys/${journey.id}/executions?limit=100`);
    if (settled.some((e) => ['RUNNING', 'WAITING'].includes(e.state)))
      throw new Error('Initial cohort still waiting; rerun after its real demo timers finish.');
    for (const [index, name] of ['Maya Patel', 'Oliver Taylor'].entries()) {
      const [first, last] = name.split(' '),
        id = name.toLowerCase().replaceAll(' ', '-');
      await call('/customers', 'POST', {
        customer_id: id,
        external_id: 'NS-' + (1013 + index),
        email: id + '@example.invalid',
        phone: '+120255501' + (12 + index),
        locale: 'en',
        timezone: 'Europe/London',
        attributes: {
          first_name: first,
          last_name: last,
          loyalty_tier: 'Member',
          total_orders: 1,
          cart_total: 89.9,
        },
        consents: { email: true, sms: true },
      });
      await call('/events', 'POST', {
        event_id: 'northstar-cart-' + id,
        customer_id: id,
        event_name: 'cart_updated',
        occurred_at: new Date().toISOString(),
        properties: {
          cart_id: 'CART-' + id,
          amount: 89.9,
          amount_display: '€89.90',
          currency: 'EUR',
        },
      });
    }
    const waitingEnd = Date.now() + 15000;
    while (Date.now() < waitingEnd) {
      const rows = await call(`/journeys/${journey.id}/executions?customer_id=maya-patel&limit=1`);
      if (rows[0]?.current_node_id === 'purchase_wait') break;
      await new Promise((r) => setTimeout(r, 500));
    }
  }
  const gate = await call(`/journeys/${journey.id}/publish-check`);
  if (gate.issues.some((issue) => issue.code === 'test_required')) {
    const saved = await call('/test-scenarios');
    for (const scenario of saved.filter((s) => s.journey_id === journey.id)) {
      const created = await call('/test-runs', 'POST', {
        journey_id: journey.id,
        scenario_id: scenario.id,
        use_current_draft: true,
      });
      const done = await call(`/test-runs/${created.id}/advance`, 'POST', {
        operation: 'complete',
        revision: created.revision,
      });
      if (done.state.result !== 'PASS')
        throw new Error('Saved showcase scenario failed current-draft regression.');
    }
  }
  const all = await call('/journeys?limit=200');
  if (all.length !== 1)
    log('Other journeys exist; use --reset before capturing a clean product gallery.');
  const runs = await call(`/test-runs?journey_id=${journey.id}&limit=200`);
  const tracked = await call(`/journeys/${journey.id}/executions?customer_id=maya-patel&limit=1`);
  const metadata = {
    workspaceId: identity.workspace_id,
    journeyId: journey.id,
    customerId: 'alex-morgan',
    monitorExecutionId: tracked[0]?.id,
    monitorCustomerId: 'maya-patel',
    customerCount: 14,
    executionCount: 14,
    synthetic: true,
    liveCadence: '5s reminder / 30s purchase window',
    draftCadence: '1h reminder / 24h purchase window',
    testRunCount: runs.length,
  };
  await mkdir('.tmp', { recursive: true });
  await writeFile('.tmp/product-demo.json', JSON.stringify(metadata, null, 2) + '\n');
  log(
    'Northstar ready: one journey, 14 customers, real Mailpit/mock-SMS activity and five passing business-cadence scenarios.',
  );
  return metadata;
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    await seedProductDemo({ reset: process.argv.includes('--reset') });
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
