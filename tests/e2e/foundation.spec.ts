import { expect, test } from '@playwright/test';

test('loads the application and confirms real database readiness', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Build journeys. Make connections.' }),
  ).toBeVisible();
  await expect(page.getByRole('status')).toHaveText('All services ready');
  await page.getByRole('button', { name: 'Check again' }).click();
  await expect(page.getByRole('status')).toHaveText('All services ready');
  const response = await page.request.get('/api/openapi.json');
  expect(response.ok()).toBeTruthy();
  expect((await response.json()).paths['/readyz']).toBeDefined();
  expect(errors).toEqual([]);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy();
});

test('shows an actionable error and recovers on retry', async ({ page }) => {
  await page.route('**/readyz', (route) => route.fulfill({ status: 503, json: { status: 503 } }));
  await page.goto('/');
  await expect(page.getByRole('status')).toHaveText('Connection unavailable');
  await expect(page.getByRole('alert')).toContainText('Check the local services');
  await page.unroute('**/readyz');
  await page.getByRole('button', { name: 'Check again' }).click();
  await expect(page.getByRole('status')).toHaveText('All services ready');
});
