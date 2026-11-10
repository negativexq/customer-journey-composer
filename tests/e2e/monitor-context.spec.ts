import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('monitor keeps the latest selected customer when an older response arrives late', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Monitor editor',
        workspace: 'Monitor context',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', {
      headers,
      data: { name: 'Monitor context journey' },
    })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart entered',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Journey completed',
        position: { x: 0, y: 170 },
        config: { reason: 'completed' },
      },
    ],
    edges: [{ id: 'a', source: 'start', target: 'done', port: 'started' }],
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
        scenario: { name: 'Monitor gate', entry_event: { event_name: 'cart_updated' } },
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
  const ids: string[] = [];
  for (const customer of ['monitor_a', 'monitor_b']) {
    expect(
      (
        await page.request.post('/api/v1/customers', { headers, data: { customer_id: customer } })
      ).ok(),
    ).toBeTruthy();
    const event = await (
      await page.request.post('/api/v1/events', {
        headers,
        data: {
          event_id: randomBytes(12).toString('hex'),
          customer_id: customer,
          event_name: 'cart_updated',
          occurred_at: new Date().toISOString(),
          properties: {},
        },
      })
    ).json();
    ids.push(event.executions[0]);
  }
  for (const id of ids)
    await expect
      .poll(async () => (await (await page.request.get('/api/v1/executions/' + id)).json()).state)
      .toBe('COMPLETED');
  await page.goto(`/journeys/${journey.id}?tab=live-monitor`);
  await expect(page.getByRole('button', { name: /monitor_b · COMPLETED/ })).toBeVisible();
  let release = () => {};
  const barrier = new Promise<void>((resolve) => {
    release = resolve;
  });
  let received = () => {};
  const held = new Promise<void>((resolve) => {
    received = resolve;
  });
  const firstURL = '**/api/v1/executions/' + ids[0];
  await page.route(firstURL, async (route) => {
    received();
    await barrier;
    await route.continue();
  });
  try {
    await page.getByRole('button', { name: /monitor_a · COMPLETED/ }).click();
    await held;
    await page.getByRole('button', { name: /monitor_b · COMPLETED/ }).click();
    const inspector = page.getByRole('region', { name: 'Execution details' });
    await expect(inspector).toContainText('monitor_b');
    const response = page.waitForResponse((r) => r.url().endsWith('/executions/' + ids[0]));
    release();
    await (await response).finished();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await expect(inspector).toContainText('monitor_b');
    await expect(inspector).not.toContainText('monitor_a');
    await expect(inspector.locator('.timeline-step')).toHaveCount(2);
    await expect(page.locator('.monitor-canvas .node-metrics')).toHaveCount(2);
    await page
      .locator('.journey-tabs')
      .getByRole('button', { name: 'Builder', exact: true })
      .click();
    await page
      .locator('.journey-tabs')
      .getByRole('button', { name: 'Live Monitor', exact: true })
      .click();
    await expect(inspector).toContainText('monitor_b');
  } finally {
    release();
  }
});
