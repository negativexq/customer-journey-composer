import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('template creates an editable draft and isolated inbox before human publishing', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Template editor',
        workspace: 'Template walkthrough',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  await page.goto('/');
  await page
    .locator('button:visible')
    .filter({ hasText: /^Templates$/ })
    .first()
    .click();
  await page.getByRole('button', { name: 'Use Abandoned Cart Recovery', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Abandoned Cart Recovery', exact: true }),
  ).toBeVisible();
  await expect(page.locator('.vue-flow__node')).toHaveCount(8);
  const journeys = await (await page.request.get('/api/v1/journeys')).json();
  const j = journeys[0];
  expect(j.status).toBe('Draft');
  // Multi-node copying preserves internal connections and is one undoable edit.
  // Clipboard actions live behind the canvas bar's More menu; shortcuts still work.
  const more = page.getByLabel('More canvas actions', { exact: true });
  await more.click();
  await page.getByRole('button', { name: 'Select all nodes', exact: true }).click();
  await more.click();
  await page.getByRole('button', { name: 'Copy selection', exact: true }).click();
  await more.click();
  await page.getByRole('button', { name: 'Paste selection', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(16);
  await expect(page.locator('.vue-flow__edge')).toHaveCount(16);
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(8);
  await expect(page.locator('.vue-flow__edge')).toHaveCount(8);
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: j.id,
        scenario: {
          name: 'Cart walkthrough',
          entry_event: { event_name: 'cart_updated', properties: { amount: 2450 } },
          customer: {
            email: 'walkthrough@example.invalid',
            phone: '+905550000001',
            attributes: { first_name: 'Ada', purchased: false },
            consents: { email: true, sms: true },
          },
          assertions: [
            { type: 'message_sent', channel: 'email' },
            { type: 'message_sent', channel: 'sms' },
            { type: 'state', expected: 'COMPLETED' },
          ],
        },
      },
    })
  ).json();
  const completed = await (
    await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
      headers,
      data: { operation: 'complete', revision: run.revision },
    })
  ).json();
  expect(completed.state.result).toBe('PASS');
  expect(completed.state.messages).toHaveLength(2);
  await page
    .locator('button:visible')
    .filter({ hasText: /^Test Inbox$/ })
    .first()
    .click();
  await expect(page.locator('.inbox-row')).toHaveCount(2);
  await page.locator('.inbox-row').filter({ hasText: 'Cart reminder email' }).click();
  await expect(page.getByTitle('Test email preview')).toBeVisible();
  expect(await (await page.request.get(`/api/v1/journeys/${j.id}/executions`)).json()).toEqual([]);
});
