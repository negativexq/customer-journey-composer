import { randomBytes, randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('refresh preserves pages, journey tabs and browser back/forward navigation', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Navigation editor',
        workspace: 'Navigation checks',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const journey = await (
    await page.request.post('/api/v1/journeys', {
      headers: { 'X-CSRF-Token': user.csrf_token },
      data: { name: 'Reloadable journey' },
    })
  ).json();
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  for (const [name, path] of [
    ['Analytics', '/analytics'],
    ['Customers', '/customers'],
    ['Events', '/events'],
    ['Test Lab', '/test-lab'],
    ['Test Inbox', '/test-inbox'],
    ['Templates', '/templates'],
    ['Settings', '/settings'],
    ['Journeys', '/journeys'],
  ]) {
    await page.getByRole('button', { name, exact: true }).filter({ visible: true }).first().click();
    await expect(page).toHaveURL(new RegExp(path + '$'));
    await page.reload();
    await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(path + '$'));
  }
  await page.getByRole('button', { name: /Reloadable journey/ }).click();
  await expect(page).toHaveURL(new RegExp(`/journeys/${journey.id}$`));
  await page
    .locator('.journey-tabs')
    .getByRole('button', { name: 'Versions', exact: true })
    .click();
  await expect(page).toHaveURL(/\?tab=versions$/);
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Published versions', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Reloadable journey', exact: true }),
  ).toBeVisible();
  await page
    .locator('.journey-tabs')
    .getByRole('button', { name: 'Analytics', exact: true })
    .click();
  await expect(page).toHaveURL(/\?tab=analytics$/);
  await page.goBack();
  await expect(
    page.getByRole('heading', { name: 'Published versions', exact: true }),
  ).toBeVisible();
  await page.goForward();
  await expect(page.getByRole('heading', { name: 'Journey analytics', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(page).toHaveURL(/\?tab=preview-test&view=test-lab$/);
  await page.reload();
  await expect(page.locator('.test-lab')).toBeVisible();
  await page.goto('/journeys');
  await page
    .locator('.list-status-tabs')
    .getByRole('button', { name: 'Active', exact: true })
    .click();
  await expect(page).toHaveURL(/\/journeys\?status=Active$/);
  await page.reload();
  await expect(
    page.locator('.list-status-tabs').getByRole('button', { name: 'Active', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true');
  await page
    .locator('.list-status-tabs')
    .getByRole('button', { name: 'Draft', exact: true })
    .click();
  await expect(page).toHaveURL(/\?status=Draft$/);
  await page.goBack();
  await expect(
    page.locator('.list-status-tabs').getByRole('button', { name: 'Active', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true');
  expect(errors).toEqual([]);
});

test('a protected deep link resumes after login and unavailable journeys fall back safely', async ({
  page,
}) => {
  const email = randomBytes(12).toString('hex') + '@example.invalid';
  const password = randomBytes(24).toString('hex');
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: { name: 'Deep link editor', workspace: 'Deep link checks', email, password },
    })
  ).json();
  const journey = await (
    await page.request.post('/api/v1/journeys', {
      headers: { 'X-CSRF-Token': user.csrf_token },
      data: { name: 'Direct journey' },
    })
  ).json();
  await page.context().clearCookies();
  await page.goto(`/journeys/${journey.id}?tab=versions`);
  await page.getByText('Local demo account is ready. Click Sign in to explore.').waitFor();
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Published versions', exact: true }),
  ).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/journeys/${journey.id}\\?tab=versions$`));
  await page.goto(`/journeys/${randomUUID()}?tab=analytics`);
  await expect(page.getByRole('heading', { name: 'Journeys', exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/journeys$/);
  await expect(page.getByRole('alert')).toBeVisible();
});
