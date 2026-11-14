import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('creates a consent-aware customer and inspects a deduplicated event', async ({ page }) => {
  const registration = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Events editor',
      workspace: 'Event workspace',
      email: `${randomBytes(10).toString('hex')}@example.invalid`,
      password: randomBytes(24).toString('hex'),
    },
  });
  expect(registration.ok()).toBeTruthy();
  await page.goto('/');
  await page
    .getByRole('button', { name: 'Customers', exact: true })
    .filter({ visible: true })
    .first()
    .click();
  await expect(page.getByText('No customers yet', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'New customer', exact: true }).click();
  await page.getByLabel('Customer ID', { exact: true }).fill('browser_customer');
  await page.getByLabel('Email', { exact: true }).fill('browser@example.invalid');
  await page.getByLabel('Timezone', { exact: true }).fill('Europe/Istanbul');
  await page.getByLabel('Email consent', { exact: true }).check();
  await page.getByLabel('Custom attributes (JSON)', { exact: true }).fill('{"first_name":"Ada"}');
  await page.getByRole('button', { name: 'Create customer', exact: true }).click();
  await expect(page.getByTestId('toast')).toContainText('Customer created');
  await expect(page.getByRole('heading', { name: 'Ada', exact: true })).toBeVisible();
  await expect(page.getByText('Email allowed', { exact: true })).toBeVisible();
  await expect(page.getByText('SMS not allowed', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'No journey entries yet' })).toBeVisible();
  await page
    .getByRole('button', { name: 'Events', exact: true })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByLabel('Customer ID', { exact: true }).fill('browser_customer');
  await page.getByRole('button', { name: 'Send event', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('ACCEPTED');
  await page.getByRole('button', { name: 'Send event', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('DUPLICATE');
  await page.locator('.event-row').first().click();
  const drawer = page.getByRole('dialog', { name: 'Event details', exact: true });
  await expect(drawer.locator('pre').first()).toContainText('cart_demo');
  await drawer.getByText('Advanced payload', { exact: true }).click();
  await expect(drawer.locator('details pre')).toContainText('"duplicate_count": 1');
});
