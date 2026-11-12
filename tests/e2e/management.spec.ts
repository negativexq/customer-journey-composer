import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('creates a workspace and persists a journey across login sessions', async ({ page }) => {
  const email = `${randomBytes(12).toString('hex')}@example.invalid`;
  const password = randomBytes(24).toString('hex');
  await page.goto('/');
  await page.getByRole('button', { name: 'Create a workspace', exact: true }).click();
  await page.getByLabel('Your name', { exact: true }).fill('Test editor');
  await page.getByLabel('Workspace name').fill('Browser workspace');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: /Good (morning|afternoon|evening), Test/ }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Explore journeys' }).click();
  await page
    .locator('.page-heading')
    .getByRole('button', { name: 'New journey', exact: true })
    .click();
  await page.getByLabel('Journey name').fill('Customer onboarding');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await page.getByRole('button', { name: 'Start from scratch', exact: false }).click();
  await expect(page.getByRole('heading', { name: 'Customer onboarding' })).toBeVisible();
  await page.getByLabel('More journey actions').click();
  await page.getByRole('button', { name: 'Journey settings', exact: true }).click();
  await page.getByLabel('Description', { exact: true }).fill('Saved across sessions');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'All changes saved' })).toBeVisible();
  const signOut = page.getByRole('button', { name: 'Sign out', exact: true });
  await signOut.filter({ visible: true }).first().click();
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('button', { name: 'Customer onboarding', exact: false }).click();
  await page.getByLabel('More journey actions').click();
  await page.getByRole('button', { name: 'Journey settings', exact: true }).click();
  await expect(page.getByLabel('Description', { exact: true })).toHaveValue(
    'Saved across sessions',
  );
});
