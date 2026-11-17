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
  await expect(page.getByRole('heading', { name: customer, exact: true })).toBeVisible();
  // The person's route is drawn on the journey map and spelled out stop by stop.
  const journeyView = page.getByRole('region', { name: 'Customer journey' });
  await expect(journeyView.getByRole('heading', { name: 'Activity journey' })).toBeVisible();
  await expect(journeyView).toContainText('Finished the journey · Version 1');
  const stops = journeyView.locator('.route-step');
  await expect(stops).toHaveCount(3);
  await expect(stops.nth(0)).toContainText('Entered when cart_updated arrived');
  await expect(stops.nth(1)).toContainText('SMS accepted by the provider');
  await expect(stops.nth(2)).toContainText('Left the journey: completed');
  await expect(journeyView.locator('.workflow-node.was-visited')).toHaveCount(3);
  await expect(journeyView.locator('.node-metrics')).toHaveCount(0);
  await page.getByRole('tab', { name: /^Activity/ }).click();
  const feed = page.getByRole('tabpanel');
  await expect(feed).toContainText('Entered Activity journey');
  await expect(feed).toContainText('SMS message');
  await expect(feed).toContainText('cart_updated');
  await page.getByRole('tab', { name: 'Profile', exact: true }).click();
  // The record is read-only until someone chooses to edit it.
  const record = page.getByRole('tabpanel');
  await expect(record).toContainText('+12025550100');
  await expect(record.getByRole('textbox')).toHaveCount(0);
  await record.getByRole('button', { name: 'Edit profile', exact: true }).click();
  await expect(page.getByLabel('Customer ID', { exact: true })).toHaveValue(customer);
  await expect(page.getByLabel('Customer ID', { exact: true })).toBeDisabled();
  await record.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(record.getByRole('textbox')).toHaveCount(0);
  await page.getByRole('tab', { name: /^Journey/ }).click();
  await journeyView.getByRole('button', { name: 'Inspect details', exact: true }).click();
  const drawer = page.getByRole('dialog', { name: 'Execution details', exact: true });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole('heading', { name: 'Activity journey' })).toBeVisible();
  await expect(drawer).toContainText('Finished the journey');
  await expect(drawer.locator('.route-step')).toHaveCount(3);
  await expect(drawer.locator('.route-step').nth(1)).toContainText('Cart reminder');
  await expect(page).toHaveURL(new RegExp(`inspect=execution&record=${executionID}`));
  await page.reload();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('button', { name: 'View entry event', exact: true }).click();
  const eventDrawer = page.getByRole('dialog', { name: 'Event details', exact: true });
  await expect(eventDrawer).toContainText('linked_cart');
  // The drawer leads back to where the person came from.
  await eventDrawer.getByRole('button', { name: 'Execution details', exact: true }).click();
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Execution details', exact: true })).toHaveCount(
    0,
  );
  await drawer.locator('.activity-message').filter({ hasText: 'SMS' }).click();
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
  await page.keyboard.press('Escape');
  await expect(drawer).toBeHidden();
  await page.goto('/events');
  // The mock SMS provider's delivery report is an event of this customer too.
  await page
    .locator('.event-row')
    .filter({ hasText: customer })
    .filter({ hasText: 'cart_updated' })
    .click();
  await expect(eventDrawer).toContainText('linked_cart');
  await page.reload();
  await expect(eventDrawer).toContainText(eventID);
});
