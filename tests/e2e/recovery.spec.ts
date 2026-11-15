import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('EUR recovery scenarios run in the sandbox with a run-scoped inline inbox', async ({
  page,
}) => {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Recovery editor',
        workspace: 'EUR recovery lab',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  await page.goto('/templates');
  await page.getByRole('button', { name: 'Use Abandoned Cart · €50+', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(14);
  await expect
    .poll(async () => {
      const entry = await page
        .locator('.vue-flow__node')
        .filter({ hasText: 'Cart abandoned · €50+' })
        .boundingBox();
      const canvas = await page.locator('.canvas').boundingBox();
      return (
        !!entry &&
        !!canvas &&
        entry.x >= canvas.x &&
        entry.x + entry.width <= canvas.x + canvas.width
      );
    })
    .toBe(true);
  const journeys = await (await page.request.get('/api/v1/journeys')).json();
  const journey = journeys[0];
  const recipe = await (await page.request.get('/api/v1/templates/abandoned-cart-eur')).json();
  expect(recipe.workflow.reentry.cooldown_seconds).toBe(604800);
  expect(recipe.scenarios).toHaveLength(5);
  for (const scenario of recipe.scenarios) {
    const response = await page.request.post('/api/v1/test-runs', {
      headers,
      data: { journey_id: journey.id, scenario },
    });
    expect(response.ok()).toBeTruthy();
    const run = await response.json();
    const completed = await (
      await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
        headers,
        data: { operation: 'complete', revision: run.revision },
      })
    ).json();
    expect(completed.state.result, scenario.name).toBe('PASS');
  }
  await page.getByRole('button', { name: 'AI composer', exact: true }).click();
  let generationRequests = 0;
  page.on('request', (request) => {
    if (request.url().includes('/ai/journeys/generate')) generationRequests++;
  });
  await page.getByRole('button', { name: 'Recover a €50+ abandoned cart', exact: false }).click();
  await expect(page.getByLabel('What should happen?')).toHaveValue(recipe.brief);
  expect(generationRequests).toBe(0);
  await page.getByRole('button', { name: 'Preview & Test', exact: true }).click();
  await page.getByRole('button', { name: 'Journey path & Test Lab', exact: true }).click();
  await expect(page.getByLabel('Recovery scenario', { exact: true })).toHaveValue('2');
  await page.getByRole('button', { name: 'Create test run', exact: true }).click();
  await page.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  const inbox = page.getByRole('region', { name: 'Test run inbox' });
  await expect(inbox.locator('.inbox-row')).toHaveCount(2);
  await inbox.locator('.inbox-row').filter({ hasText: 'EMAIL' }).click();
  await expect(
    inbox.frameLocator('iframe[title="Test email preview"]').getByText(/items worth €89.90/),
  ).toBeVisible();
  await inbox.locator('.inbox-row').filter({ hasText: 'SMS' }).click();
  await expect(inbox.locator('.sms-phone')).toContainText('COMEBACK10');
  await expect(inbox.locator('.sms-phone')).toContainText('€89.90');
  await inbox.getByRole('button', { name: 'Decision', exact: true }).click();
  await expect(inbox.getByText('Eligible to send', { exact: true })).toBeVisible();
  await page.locator('.journey-tabs').getByRole('button', { name: 'Builder', exact: true }).click();
  await page
    .locator('.journey-tabs')
    .getByRole('button', { name: 'Preview & Test', exact: true })
    .click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await expect(inbox.locator('.inbox-row')).toHaveCount(2);
  await expect(page.getByLabel('Recovery scenario', { exact: true })).toHaveValue('2');
  await page.getByLabel('Recovery scenario', { exact: true }).selectOption('3');
  await expect(inbox).toBeHidden();
  await page.getByRole('button', { name: 'Create test run', exact: true }).click();
  await page.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await expect(inbox.locator('.inbox-row')).toHaveCount(1);
  await expect(inbox.locator('.inbox-row')).toContainText('Email ·');
  expect(
    await (await page.request.get(`/api/v1/journeys/${journey.id}/executions`)).json(),
  ).toEqual([]);
  expect(await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()).toMatchObject({
    status: 'Draft',
  });
});
