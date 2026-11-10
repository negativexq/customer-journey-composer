import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('builds a connected workflow, validates and publishes an immutable version', async ({
  page,
}) => {
  const email = `${randomBytes(12).toString('hex')}@example.invalid`;
  const password = randomBytes(24).toString('hex');
  const registration = await page.request.post('/api/v1/auth/register', {
    data: { name: 'Builder', workspace: 'Canvas workspace', email, password },
  });
  expect(registration.ok()).toBeTruthy();
  const identity = await registration.json();
  const creation = await page.request.post('/api/v1/journeys', {
    headers: { 'X-CSRF-Token': identity.csrf_token },
    data: { name: 'Canvas journey' },
  });
  const journey = await creation.json();
  await page.goto('/');
  await page.getByRole('button', { name: 'Canvas journey', exact: false }).click();
  await page.getByRole('button', { name: '+ Event Trigger', exact: true }).click();
  await page.getByRole('button', { name: '+ Exit', exact: true }).click();
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(2);
  await page.locator('.vue-flow__handle.source').dragTo(page.locator('.vue-flow__handle.target'));
  await expect(page.locator('.vue-flow__edge')).toHaveCount(1);
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: 'Publish', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Journey published');
  const response = await page.request.get(`/api/v1/journeys/${journey.id}/versions`);
  const versions = await response.json();
  expect(versions).toHaveLength(1);
  const detail = await page.request.get(`/api/v1/journeys/${journey.id}/versions/1`);
  expect((await detail.json()).workflow.nodes).toHaveLength(2);
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const customer = await page.request.post('/api/v1/customers', {
    headers,
    data: { customer_id: 'cart_customer' },
  });
  expect(customer.ok()).toBeTruthy();
  const event = await page.request.post('/api/v1/events', {
    headers,
    data: {
      event_id: randomBytes(12).toString('hex'),
      customer_id: 'cart_customer',
      event_name: 'cart_updated',
      occurred_at: new Date().toISOString(),
      properties: { amount: 2450 },
    },
  });
  expect(event.ok()).toBeTruthy();
  const result = await event.json();
  expect(result.executions).toHaveLength(1);
  await expect
    .poll(async () => {
      const response = await page.request.get(`/api/v1/executions/${result.executions[0]}`);
      return (await response.json()).state;
    })
    .toBe('COMPLETED');
  await page.getByRole('button', { name: 'Live Monitor', exact: true }).click();
  await page.getByLabel('Track customer ID').fill('cart_customer');
  await page.getByRole('button', { name: 'Track customer', exact: true }).click();
  await page.getByRole('button', { name: /cart_customer · COMPLETED/ }).click();
  await expect(page.getByRole('heading', { name: 'Execution inspector' })).toBeVisible();
  await expect(page.locator('.monitor-canvas .vue-flow__node')).toHaveCount(2);
  await expect
    .poll(async () => {
      const result = await (await page.request.get(`/api/v1/journeys/${journey.id}/stats`)).json();
      return result.metrics.find((m: { metric: string }) => m.metric === 'entries')?.value;
    })
    .toBe(1);
  await page
    .locator('.detail-tabs')
    .getByRole('button', { name: 'Analytics', exact: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Channel performance · logical sends' }),
  ).toBeVisible();
  await page.reload();
  await page.getByRole('button', { name: 'Canvas journey', exact: false }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(2);
});
