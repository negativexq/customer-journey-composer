import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('entry identity matching persists and scopes the purchase waiter', async ({ page }) => {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Correlation test',
        workspace: 'Correlation test',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const j = await (
    await page.request.post('/api/v1/templates/abandoned-cart/create-journey', {
      headers,
      data: {},
    })
  ).json();
  const entries = [
    {
      kind: 'event',
      event_name: 'cart_updated',
      label: 'Cart entry',
      data_type: 'event',
      enabled: true,
      aliases: [],
    },
    {
      kind: 'event',
      event_name: 'purchase_completed',
      label: 'Purchase',
      data_type: 'event',
      enabled: true,
      aliases: [],
    },
    ...['cart_updated', 'purchase_completed'].map((event_name) => ({
      kind: 'event_field',
      event_name,
      path: 'event.properties.cart_id',
      label: 'Cart identifier',
      data_type: 'string',
      enabled: true,
      aliases: [],
    })),
  ];
  expect(
    (await page.request.put('/api/v1/data-catalog', { headers, data: { entries } })).ok(),
  ).toBeTruthy();
  await page.goto('/journeys/' + j.id);
  await page.getByLabel('More journey actions', { exact: true }).click();
  await page.getByRole('button', { name: 'Journey settings', exact: true }).click();
  await page.getByRole('combobox', { name: 'Exit matching', exact: true }).click();
  await page
    .getByRole('option', { name: 'Only the object that started this journey', exact: true })
    .click();
  for (const label of ['Entry identifier', 'Incoming identifier']) {
    await page.getByRole('combobox', { name: label, exact: true }).click();
    await page.getByRole('option', { name: 'Cart identifier', exact: true }).click();
  }
  const correlation = {
    entry_path: 'event.properties.cart_id',
    event_path: 'event.properties.cart_id',
  };
  await expect
    .poll(
      async () =>
        (await (await page.request.get('/api/v1/journeys/' + j.id)).json()).draft.global_exit
          .correlation,
    )
    .toEqual(correlation);
  await page.reload();
  await page.getByLabel('More journey actions', { exact: true }).click();
  await page.getByRole('button', { name: 'Journey settings', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Exit matching', exact: true })).toContainText(
    'Only the object',
  );
  await expect(page.getByRole('combobox', { name: 'Entry identifier', exact: true })).toContainText(
    'Cart identifier',
  );
  for (const cart of ['A', 'B']) {
    const scenario = {
      name: 'Purchase ' + cart,
      start_time: '2026-11-12T10:00:00Z',
      customer: {
        customer_id: 'fictional',
        email: 'alex@example.invalid',
        phone: '+15005550006',
        attributes: { first_name: 'Alex', purchased: false },
        consents: { email: true, sms: true },
      },
      entry_event: { event_id: 'entry', event_name: 'cart_updated', properties: { cart_id: 'A' } },
      events: [
        {
          after_seconds: 10800,
          event: {
            event_id: 'purchase',
            event_name: 'purchase_completed',
            properties: { cart_id: cart },
          },
        },
      ],
      frequency_limit: 20,
      frequency_window_seconds: 86400,
      assertions: [
        { type: 'exit_reason', expected: cart === 'A' ? 'converted' : 'completed' },
        { type: 'message_count', channel: 'email', expected: 1 },
        { type: 'message_count', channel: 'sms', expected: cart === 'A' ? 0 : 1 },
      ],
    };
    const run = await (
      await page.request.post('/api/v1/test-runs', {
        headers,
        data: { journey_id: j.id, scenario },
      })
    ).json();
    const result = await (
      await page.request.post('/api/v1/test-runs/' + run.id + '/advance', {
        headers,
        data: { operation: 'complete', revision: run.revision },
      })
    ).json();
    expect(result.state.result).toBe('PASS');
  }
  await page.getByRole('combobox', { name: 'Exit matching', exact: true }).click();
  await page
    .getByRole('option', { name: 'Any matching event from this customer', exact: true })
    .click();
  await expect
    .poll(
      async () =>
        (await (await page.request.get('/api/v1/journeys/' + j.id)).json()).draft.global_exit
          .correlation,
    )
    .toBeUndefined();
});
