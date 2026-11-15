import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { choose } from './helpers/select';
test('virtual clock runs persisted sandbox without changing production', async ({ page }) => {
  const signup = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Test runner',
      workspace: 'Test Lab',
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    },
  });
  const user = await signup.json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await page.request.post('/api/v1/journeys', {
    headers,
    data: { name: 'Virtual cart' },
  });
  const j = await created.json();
  const nodes = [
    { id: 'start', type: 'event_trigger', label: 'Cart', config: { event_name: 'cart_updated' } },
    { id: 'wait', type: 'wait', label: 'Wait 26h', config: { duration_seconds: 93600 } },
    {
      id: 'email',
      type: 'email',
      label: 'Reminder',
      config: { subject: 'Your cart', text: 'Hello {{ customer.first_name }}' },
    },
    { id: 'done', type: 'exit', label: 'Complete', config: { reason: 'completed' } },
  ];
  const edges = [
    { id: 'a', source: 'start', target: 'wait', port: 'started' },
    { id: 'b', source: 'wait', target: 'email', port: 'completed' },
    { id: 'c', source: 'email', target: 'done', port: 'next' },
  ];
  expect(
    (
      await page.request.put(`/api/v1/journeys/${j.id}/draft`, {
        headers,
        data: {
          name: j.name,
          description: '',
          tags: [],
          revision: j.revision,
          draft: {
            schema_version: 1,
            nodes,
            edges,
            global_exit: { event_name: 'purchase_completed' },
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto('/');
  await page.getByRole('button', { name: 'Virtual cart', exact: false }).click();
  await page.getByRole('button', { name: 'Preview & Test', exact: true }).click();
  await page.getByRole('button', { name: 'Journey path & Test Lab', exact: true }).click();
  await page.getByRole('button', { name: 'Create test run', exact: true }).click();
  await page.getByRole('button', { name: 'Run', exact: true }).click();
  await expect(page.locator('.test-lab')).toContainText('WAITING');
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.locator('.test-lab')).toContainText('paused');
  await page.getByRole('button', { name: 'Resume', exact: true }).click();
  await page.getByRole('button', { name: 'Advance 1d', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PENDING');
  await page.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await expect(page.locator('.test-lab')).toContainText('SIMULATED');
  const runs = await (await page.request.get(`/api/v1/test-runs?journey_id=${j.id}`)).json();
  expect(runs).toHaveLength(1);
  expect(runs[0].message_count).toBe(1);
  expect(await (await page.request.get(`/api/v1/journeys/${j.id}/executions`)).json()).toEqual([]);
  await page.getByRole('button', { name: 'Reset / Replay', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PENDING');
  await page.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await page.getByText('Advanced scenario options', { exact: false }).click();
  await page
    .getByLabel('Event sequence (JSON; after_seconds, event)')
    .fill('[{"after_seconds":3600,"event":{"event_name":"purchase_completed"}}]');
  await page
    .getByLabel('Assertions (JSON)')
    .fill('[{"type":"state","expected":"EXITED"},{"type":"no_message","channel":"email"}]');
  await page.getByRole('button', { name: 'Preview journey path', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await expect(page.locator('.test-lab')).toContainText('No messages produced.');
  await page
    .locator('button:visible')
    .filter({ hasText: /^Test Inbox$/ })
    .first()
    .click();
  await expect(page.getByRole('heading', { name: 'Test Inbox', exact: true })).toBeVisible();
  await expect(page.locator('.inbox-row')).toHaveCount(2);
  await page.locator('.inbox-row').first().click();
  await expect(page.getByTitle('Test email preview')).toBeVisible();
  await page.getByRole('button', { name: 'Attempts', exact: true }).click();
  await expect(page.locator('.inbox-detail')).toContainText('ACCEPTED');
  await page.getByRole('button', { name: 'Compare with previous run', exact: true }).click();
  await expect(page.locator('.inbox-detail pre')).toContainText('SIMULATED');
  await choose(page.getByLabel('Status', { exact: true }), 'Blocked');
  await expect(page.locator('.inbox-row')).toHaveCount(0);
});
