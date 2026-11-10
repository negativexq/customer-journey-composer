import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('renders personalized previews and sends isolated test email to Mailpit', async ({
  page,
  playwright,
}) => {
  const seed = randomBytes(10).toString('hex');
  const email = `${seed}@example.invalid`;
  const registration = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Message editor',
      workspace: 'Messaging workspace',
      email,
      password: randomBytes(24).toString('hex'),
    },
  });
  expect(registration.ok()).toBeTruthy();
  const identity = await registration.json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const policy = await page.request.put('/api/v1/settings/messaging', {
    headers,
    data: { frequency_limit: 3, frequency_window_seconds: 86400, test_allowlist: [email] },
  });
  expect(policy.ok()).toBeTruthy();
  const create = await page.request.post('/api/v1/journeys', {
    headers,
    data: { name: 'Message journey' },
  });
  const journey = await create.json();
  const draft = {
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
        id: 'mail',
        type: 'email',
        label: 'Cart email',
        position: { x: 0, y: 160 },
        config: {
          subject: `Journey-${seed} {{ customer.first_name }}`,
          preheader: 'A little reminder',
          html: '<p>Hello {{ customer.first_name }}, your cart is waiting.</p>',
          text: 'Hello {{ customer.first_name }}, your cart is waiting.',
        },
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
      { id: 'a', source: 'start', target: 'mail', port: 'started' },
      { id: 'b', source: 'mail', target: 'done', port: 'next' },
    ],
  };
  const saved = await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
    headers,
    data: { name: journey.name, draft, revision: journey.revision },
  });
  expect(saved.ok()).toBeTruthy();
  await page.goto('/');
  await page.getByRole('button', { name: 'Message journey', exact: false }).click();
  await page.getByRole('button', { name: 'Preview & Test', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: `Journey-${seed} Ada`, exact: true }),
  ).toBeVisible();
  await expect(
    page
      .frameLocator('iframe[title="Isolated email preview"]')
      .getByText('Hello Ada, your cart is waiting.'),
  ).toBeVisible();
  await page.getByLabel('Test recipient', { exact: true }).fill(email);
  await page.getByRole('button', { name: 'Send test email', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Test accepted' })).toBeVisible({
    timeout: 15000,
  });
  const jobs = await (await page.request.get('/api/v1/deliveries')).json();
  expect(jobs).toHaveLength(1);
  expect(jobs[0].mode).toBe('TEST');
  expect(jobs[0].status).toBe('ACCEPTED');
  const inbox = await (await page.request.get('/api/v1/test-inbox?status=TEST_ACCEPTED')).json();
  expect(inbox).toHaveLength(1);
  expect(inbox[0].mode).toBe('REAL_TEST');
  expect(inbox[0].delivery_id).toBe(jobs[0].id);
  const mailpit = await playwright.request.newContext({
    baseURL: process.env.E2E_MAILPIT_URL || 'http://127.0.0.1:8025',
  });
  try {
    await expect
      .poll(async () => {
        const response = await mailpit.get(
          `/api/v1/search?query=${encodeURIComponent(`subject:Journey-${seed}`)}`,
        );
        return (await response.json()).messages_count;
      })
      .toBe(1);
  } finally {
    await mailpit.dispose();
  }
});

test('live nodes send through SMTP and mock SMS with visible attempts', async ({ page }) => {
  const seed = randomBytes(10).toString('hex');
  const signup = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Live editor',
      workspace: 'Delivery workspace',
      email: `${seed}@example.invalid`,
      password: randomBytes(24).toString('hex'),
    },
  });
  const user = await signup.json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Channel execution' } })
  ).json();
  const nodes = [
    {
      id: 'start',
      type: 'event_trigger',
      label: 'Cart',
      position: { x: 0, y: 0 },
      config: { event_name: 'cart_updated' },
    },
    {
      id: 'email',
      type: 'email',
      label: 'Email',
      position: { x: 0, y: 150 },
      config: { subject: `Live-${seed}`, text: 'Hello {{ customer.first_name }}' },
    },
    {
      id: 'sms',
      type: 'sms',
      label: 'SMS',
      position: { x: 0, y: 300 },
      config: { text: 'Hello {{ customer.first_name }}' },
    },
    {
      id: 'done',
      type: 'exit',
      label: 'Done',
      position: { x: 0, y: 450 },
      config: { reason: 'completed' },
    },
  ];
  const edges = [
    { id: 'a', source: 'start', target: 'email', port: 'started' },
    { id: 'b', source: 'email', target: 'sms', port: 'next' },
    { id: 'c', source: 'sms', target: 'done', port: 'next' },
  ];
  const draft = await (
    await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
      headers,
      data: { name: journey.name, draft: { schema_version: 1, nodes, edges }, revision: 1 },
    })
  ).json();
  const gate = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: {
          name: 'Send gate',
          entry_event: { event_name: 'cart_updated' },
          customer: {
            email: 'gate@example.invalid',
            phone: '+905551234567',
            attributes: { first_name: 'Ada' },
            consents: { email: true, sms: true },
          },
        },
      },
    })
  ).json();
  expect(
    (
      await page.request.post(`/api/v1/test-runs/${gate.id}/advance`, {
        headers,
        data: { operation: 'complete', revision: gate.revision },
      })
    ).ok(),
  ).toBeTruthy();
  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: draft.revision },
  });
  expect(published.ok()).toBeTruthy();
  await page.request.post('/api/v1/customers', {
    headers,
    data: {
      customer_id: 'delivery_customer',
      email: `live-${seed}@example.invalid`,
      phone: '+905551234567',
      attributes: { first_name: 'Ada' },
      consents: { email: true, sms: true },
    },
  });
  const event = await page.request.post('/api/v1/events', {
    headers,
    data: {
      event_id: seed,
      customer_id: 'delivery_customer',
      event_name: 'cart_updated',
      occurred_at: new Date().toISOString(),
    },
  });
  expect(event.ok()).toBeTruthy();
  await expect
    .poll(
      async () => {
        const jobs = await (
          await page.request.get('/api/v1/deliveries?customer_id=delivery_customer')
        ).json();
        return jobs.filter((j: { status: string }) => j.status === 'ACCEPTED').length;
      },
      { timeout: 15000 },
    )
    .toBe(2);
  const jobs = await (
    await page.request.get('/api/v1/deliveries?customer_id=delivery_customer')
  ).json();
  expect(jobs.map((j: { provider: string }) => j.provider).sort()).toEqual(['mock_sms', 'smtp']);
  const sandboxInbox = await (await page.request.get('/api/v1/test-inbox')).json();
  expect(sandboxInbox).toHaveLength(2);
  expect(sandboxInbox.every((m: { mode: string }) => m.mode === 'SIMULATED')).toBeTruthy();
  for (const job of jobs) {
    const detail = await (await page.request.get(`/api/v1/deliveries/${job.id}`)).json();
    expect(detail.attempt_history).toHaveLength(1);
    expect(detail.message.text).toContain('Ada');
    expect(detail.mode).toBe('LIVE');
  }
});
