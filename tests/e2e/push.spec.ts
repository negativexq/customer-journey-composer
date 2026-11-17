import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

const shots = process.env.PUSH_SHOTS;

test('a push step previews on the phone, lands in the Test Inbox and reaches the mock provider', async ({
  page,
}) => {
  const seed = randomBytes(8).toString('hex');
  const registration = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Push editor',
      workspace: 'Northwind Store',
      email: `${seed}@example.invalid`,
      password: randomBytes(24).toString('hex'),
    },
  });
  expect(registration.ok()).toBeTruthy();
  const identity = await registration.json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Push journey' } })
  ).json();
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
        id: 'nudge',
        type: 'push',
        label: 'Cart push',
        position: { x: 0, y: 160 },
        config: {
          subject: '{{ customer.attributes.first_name }}, your cart is waiting',
          text: 'The items you picked are still in your cart. Tap to finish checking out.',
          url: 'https://shop.example/cart',
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
      { id: 'a', source: 'start', target: 'nudge', port: 'started' },
      { id: 'b', source: 'nudge', target: 'done', port: 'next' },
    ],
  };
  const saved = await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
    headers,
    data: { name: journey.name, draft, revision: journey.revision },
  });
  expect(saved.ok(), await saved.text()).toBeTruthy();
  const current = await saved.json();

  // The studio previews the notification as it drops over the phone's screen.
  await page.goto(`/journeys/${journey.id}`);
  await page.getByRole('button', { name: 'Preview & Test', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Push studio', exact: true })).toBeVisible();
  const studioCard = page.getByRole('article', { name: 'Push notification' });
  await expect(studioCard).toContainText('Ada, your cart is waiting');
  await expect(studioCard).toContainText('Northwind Store');
  await expect(page.getByText('Tapping opens shop.example/cart')).toBeVisible();
  if (shots)
    await page
      .locator('.push-preview')
      .screenshot({ animations: 'disabled', path: `${shots}/studio-banner.png` });

  // A sandbox run puts the notification in the Test Inbox.
  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: {
          name: 'Push sandbox',
          customer: {
            customer_id: 'push_sandbox',
            push_token: 'sandbox-device-token',
            attributes: { first_name: 'Ada' },
            consents: { push: true },
          },
          entry_event: { event_name: 'cart_updated' },
        },
      },
    })
  ).json();
  const advanced = await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
    headers,
    data: { operation: 'complete', revision: run.revision },
  });
  expect(advanced.ok(), await advanced.text()).toBeTruthy();
  await page.goto('/test-inbox');
  await page.locator('.inbox-row').first().click();
  await expect(page.locator('.inbox-row').first()).toContainText('Push');
  const inboxCard = page.locator('.inbox-detail').getByRole('article', {
    name: 'Push notification',
  });
  await expect(inboxCard).toContainText('Ada, your cart is waiting');
  if (shots)
    await page
      .locator('.push-preview')
      .screenshot({ animations: 'disabled', path: `${shots}/inbox-banner.png` });
  await page.locator('.inbox-detail').getByRole('radio', { name: 'Lock screen' }).click();
  await expect(page.locator('.push-phone.lock')).toBeVisible();
  await expect(inboxCard).toContainText('The items you picked are still in your cart.');
  if (shots)
    await page
      .locator('.push-preview')
      .screenshot({ animations: 'disabled', path: `${shots}/inbox-lock.png` });

  // Live: the worker sends it through the mock push provider to the customer's device token.
  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: current.revision },
  });
  expect(published.ok(), await published.text()).toBeTruthy();
  const customer = await page.request.post('/api/v1/customers', {
    headers,
    data: {
      customer_id: 'push_live',
      push_token: 'device-token-live',
      attributes: { first_name: 'Ada' },
      consents: { push: true },
    },
  });
  expect(customer.ok(), await customer.text()).toBeTruthy();
  const entry = await page.request.post('/api/v1/events', {
    headers,
    data: {
      event_id: randomBytes(12).toString('hex'),
      customer_id: 'push_live',
      event_name: 'cart_updated',
      occurred_at: new Date().toISOString(),
    },
  });
  expect(entry.ok(), await entry.text()).toBeTruthy();
  await expect
    .poll(
      async () => {
        const rows = await (
          await page.request.get('/api/v1/deliveries?customer_id=push_live')
        ).json();
        return rows.map((r: { channel: string; provider: string; status: string }) =>
          [r.channel, r.provider, r.status].join(' '),
        );
      },
      { timeout: 30_000 },
    )
    .toEqual(['push mock_push ACCEPTED']);
});
