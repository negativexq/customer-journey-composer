import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('a wait can end at a time of day or relative to a date on the customer', async ({ page }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Scheduler',
        workspace: 'Wait types',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Renewal reminder' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Subscribed',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'hold',
        type: 'wait',
        label: 'Hold',
        position: { x: 0, y: 190 },
        config: { duration_seconds: 3600, timezone: 'UTC' },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 380 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'hold', port: 'started' },
      { id: 'b', source: 'hold', target: 'done', port: 'completed' },
    ],
  };
  const journey = await (
    await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
      headers,
      data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
    })
  ).json();
  const hold = async () =>
    (await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()).draft.nodes.find(
      (node: { id: string }) => node.id === 'hold',
    ).config;

  await page.goto(`/journeys/${journey.id}`);
  const card = page.locator('.canvas .vue-flow__node[data-id="hold"]');
  await expect(card.locator('.workflow-node-detail')).toHaveText('Wait 1 hour');
  await card.click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await settings.getByLabel('Wait type', { exact: true }).selectOption('time');
  await expect(settings.getByLabel('Wait for', { exact: true })).toHaveCount(0);
  await settings.getByLabel('Time of day', { exact: true }).fill('09:30');
  await settings.getByRole('button', { name: 'Tue', exact: true }).click();
  await settings.getByRole('button', { name: 'Mon', exact: true }).click();
  await expect(card.locator('.workflow-node-detail')).toHaveText('Until 09:30 on Mon, Tue');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  expect(await hold()).toEqual({
    wait_mode: 'time',
    at_time: '09:30',
    weekdays: [1, 2],
    timezone: 'UTC',
  });

  await settings.getByLabel('Wait type', { exact: true }).selectOption('date');
  await settings.getByLabel('Date field', { exact: true }).fill('customer.attributes.renews_on');
  await settings.getByLabel('Offset', { exact: true }).fill('3');
  await expect(card.locator('.workflow-node-detail')).toHaveText('Until 3 days before renews on');
  await settings.getByLabel('Direction', { exact: true }).selectOption('1');
  await expect(card.locator('.workflow-node-detail')).toHaveText('Until 3 days after renews on');
  await settings.getByLabel('Direction', { exact: true }).selectOption('-1');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  // Switching type leaves none of the previous type's fields behind.
  expect(await hold()).toEqual({
    wait_mode: 'date',
    date_path: 'customer.attributes.renews_on',
    offset_seconds: -259200,
    timezone: 'UTC',
  });

  // The sandbox clock stops exactly three days before the customer's date.
  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: {
          name: 'Renewal date',
          start_time: '2030-01-01T00:00:00Z',
          customer: { customer_id: 'renewer', attributes: { renews_on: '2030-01-10' } },
          entry_event: { event_name: 'cart_updated' },
        },
      },
    })
  ).json();
  const finished = await (
    await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
      headers,
      data: { operation: 'complete', revision: run.revision },
    })
  ).json();
  expect(finished.state.status).toBe('COMPLETED');
  expect(new Date(finished.state.now).toISOString()).toBe('2030-01-07T00:00:00.000Z');
});
