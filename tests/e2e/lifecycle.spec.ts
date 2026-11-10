import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('pause and sunset preserve durable work while closing enrollment', async ({ page }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Operator',
        workspace: 'Lifecycle',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const j = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Lifecycle journey' } })
  ).json();
  const spec = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Start',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'delay',
        type: 'wait',
        label: 'Wait',
        position: { x: 0, y: 180 },
        config: { duration_seconds: 3 },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 360 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'delay', port: 'started' },
      { id: 'b', source: 'delay', target: 'done', port: 'completed' },
    ],
  };
  const saved = await (
    await page.request.put(`/api/v1/journeys/${j.id}/draft`, {
      headers,
      data: { name: j.name, draft: spec, revision: 1 },
    })
  ).json();
  const gate = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: j.id,
        scenario: { name: 'Lifecycle gate', entry_event: { event_name: 'cart_updated' } },
      },
    })
  ).json();
  await page.request.post(`/api/v1/test-runs/${gate.id}/advance`, {
    headers,
    data: { operation: 'complete', revision: gate.revision },
  });
  expect(
    (
      await page.request.post(`/api/v1/journeys/${j.id}/publish`, {
        headers,
        data: { revision: saved.revision },
      })
    ).ok(),
  ).toBeTruthy();
  for (const id of ['existing', 'new'])
    await page.request.post('/api/v1/customers', { headers, data: { customer_id: id } });
  const send = async (id: string) =>
    (
      await page.request.post('/api/v1/events', {
        headers,
        data: {
          event_id: randomBytes(12).toString('hex'),
          event_name: 'cart_updated',
          customer_id: id,
          occurred_at: new Date().toISOString(),
        },
      })
    ).json();
  const entry = await send('existing');
  const state = async () =>
    (await (await page.request.get('/api/v1/executions/' + entry.executions[0])).json()).state;
  await expect.poll(state).toBe('WAITING');
  await page.goto('/');
  await page.getByRole('button', { name: 'Lifecycle journey', exact: false }).click();
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Resume', exact: true })).toBeVisible();
  expect((await send('new')).executions).toEqual([]);
  // Let a real durable timer become due while paused; no worker step may run.
  await page.waitForTimeout(3200);
  expect(await state()).toBe('WAITING');
  await page.getByRole('button', { name: 'Sunset', exact: true }).click();
  await expect.poll(state).toBe('COMPLETED');
  expect((await send('new')).executions).toEqual([]);
  await page.getByRole('button', { name: 'Complete after draining', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('completed');
  await page.getByRole('button', { name: 'Archive', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('archived');
  const audits = await (await page.request.get('/api/v1/audit-logs')).json();
  expect(audits.some((a: { action: string }) => a.action === 'journey.pause')).toBeTruthy();
});
