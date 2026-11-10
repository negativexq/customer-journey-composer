import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('analytics filters support search, keyboard selection and real period requests', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Reports editor',
        workspace: 'Reporting filters',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  for (const name of ['Cart recovery', 'Welcome series']) {
    expect(
      (
        await page.request.post('/api/v1/journeys', {
          headers: { 'X-CSRF-Token': user.csrf_token },
          data: { name, description: '', tags: [] },
        })
      ).ok(),
    ).toBeTruthy();
  }
  await page.goto('/');
  await page
    .getByRole('button', { name: 'Analytics', exact: true })
    .filter({ visible: true })
    .first()
    .click();
  const journeyPicker = page.getByRole('button', { name: 'Journey: All journeys', exact: true });
  await expect(journeyPicker).toBeEnabled();
  await journeyPicker.click();
  await page.getByRole('combobox', { name: 'Find a journey…' }).fill('cart');
  await expect(page.getByRole('option')).toHaveCount(1);
  await page.getByRole('combobox', { name: 'Find a journey…' }).press('Enter');
  await expect(
    page.getByRole('button', { name: 'Journey: Cart recovery', exact: true }),
  ).toBeEnabled();
  const period = page.getByRole('button', { name: 'Trend period: Last 30 days', exact: true });
  await period.click();
  await expect(page.getByRole('option', { name: /Last 7 days/ })).toBeVisible();
  const response = page.waitForResponse((r) => r.url().includes('/analytics?days=7') && r.ok());
  await page.getByRole('listbox', { name: 'Trend period', exact: true }).press('Home');
  await page.getByRole('listbox', { name: 'Trend period', exact: true }).press('Enter');
  await response;
  await expect(
    page.getByRole('button', { name: 'Trend period: Last 7 days', exact: true }),
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Journey: Cart recovery', exact: true }).click();
  await page.getByRole('combobox', { name: 'Find a journey…' }).press('Escape');
  await expect(page.getByRole('dialog', { name: 'Journey', exact: true })).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Journey: Cart recovery', exact: true }),
  ).toBeFocused();
  await expect(page.getByText('All-time totals', { exact: true })).toBeVisible();
});
