import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('customer, event, execution and message inspections survive reload and browser history', async ({
  page,
  request,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Activity editor',
        workspace: 'Activity links',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const customer = 'linked_customer';
  const eventID = randomBytes(12).toString('hex');
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Activity journey' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart entry',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'sms',
        type: 'sms',
        label: 'Cart reminder',
        position: { x: 0, y: 170 },
        config: { text: 'Your cart is waiting.' },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Complete',
        position: { x: 0, y: 340 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'sms', port: 'started' },
      { id: 'b', source: 'sms', target: 'done', port: 'next' },
    ],
  };
  const journey = await (
    await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
      headers,
      data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
    })
  ).json();
  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: {
          name: 'Activity gate',
          customer: { customer_id: customer, phone: '+12025550100', consents: { sms: true } },
          entry_event: { event_name: 'cart_updated' },
        },
      },
    })
  ).json();
  await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
    headers,
    data: { operation: 'complete', revision: run.revision },
  });
  expect(
    (
      await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
        headers,
        data: { revision: journey.revision },
      })
    ).ok(),
  ).toBeTruthy();
  await page.request.post('/api/v1/customers', {
    headers,
    data: { customer_id: customer, phone: '+12025550100', consents: { sms: true } },
  });
  const entry = await (
    await page.request.post('/api/v1/events', {
      headers,
      data: {
        event_id: eventID,
        customer_id: customer,
        event_name: 'cart_updated',
        occurred_at: new Date().toISOString(),
        properties: { cart_id: 'linked_cart' },
      },
    })
  ).json();
  const executionID = entry.executions[0];
  await expect
    .poll(
      async () =>
        (await (await page.request.get(`/api/v1/executions/${executionID}`)).json()).state,
    )
    .toBe('COMPLETED');
  await expect
    .poll(
      async () =>
        (await (await page.request.get(`/api/v1/deliveries?customer_id=${customer}`)).json())[0]
          ?.status,
    )
    .toBe('ACCEPTED');
  const delivery = (
    await (await page.request.get(`/api/v1/deliveries?customer_id=${customer}`)).json()
  )[0];
  const fromEvent = await (await page.request.get(`/api/v1/events/${eventID}/executions`)).json();
  expect(fromEvent).toHaveLength(1);
  expect(fromEvent[0].id).toBe(executionID);
  await request.post('/api/v1/auth/register', {
    data: {
      name: 'Other workspace',
      workspace: 'Other activity',
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    },
  });
  for (const path of [`/events/${eventID}/executions`, `/customers/${customer}/executions`])
    expect(await (await request.get('/api/v1' + path)).json()).toEqual([]);
  for (const path of [
    `/events/${eventID}`,
    `/executions/${executionID}`,
    `/deliveries/${delivery.id}`,
  ])
    expect((await request.get('/api/v1' + path)).status()).toBe(404);
  await page.goto(`/customers?customer=${customer}`);
  await expect(page.getByLabel('Customer ID', { exact: true })).toHaveValue(customer);
  await page
    .getByRole('button')
    .filter({ hasText: executionID.slice(0, 8) })
    .click();
  const drawer = page.getByRole('dialog', { name: 'Execution details', exact: true });
  await expect(drawer).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`inspect=execution&record=${executionID}`));
  await page.reload();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('button', { name: 'View entry event', exact: true }).click();
  const eventDrawer = page.getByRole('dialog', { name: 'Event details', exact: true });
  await expect(eventDrawer).toContainText('linked_cart');
  await page.goBack();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('button').filter({ hasText: 'sms · SMS' }).click();
  const messageDrawer = page.getByRole('dialog', { name: 'Message details', exact: true });
  await expect(messageDrawer).toContainText('Your cart is waiting.');
  await expect(messageDrawer).toContainText('Eligible to send');
  await page.reload();
  await expect(messageDrawer).toContainText('Your cart is waiting.');
  await page.goBack();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('button', { name: 'Open in Live Monitor', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`tab=live-monitor&execution=${executionID}`));
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText(customer);
  await page.reload();
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText(executionID);
  await page.getByRole('button', { name: 'Open activity inspector', exact: true }).click();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('button', { name: 'Close activity inspector', exact: true }).click();
  await page.goto('/events');
  await page.locator('.event-row').filter({ hasText: customer }).click();
  await expect(eventDrawer).toContainText('linked_cart');
  await page.reload();
  await expect(eventDrawer).toContainText(eventID);
});
