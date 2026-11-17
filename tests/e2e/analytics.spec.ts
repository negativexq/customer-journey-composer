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
  const period = page.getByRole('radiogroup', { name: 'Trend period', exact: true });
  await expect(period.getByRole('radio', { name: '30 days', exact: true })).toBeChecked();
  const response = page.waitForResponse((r) => r.url().includes('/analytics?days=7') && r.ok());
  await period.getByRole('radio', { name: '7 days', exact: true }).click();
  await response;
  await expect(period.getByRole('radio', { name: '7 days', exact: true })).toBeChecked();
  // A custom range is picked on a calendar and applied as one request.
  await period.getByRole('radio', { name: 'Custom', exact: true }).click();
  const custom = page.getByRole('dialog', { name: 'Custom range', exact: true });
  await expect(custom.getByRole('button', { name: 'Apply', exact: true })).toBeEnabled();
  await custom.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(custom).toHaveCount(0);
  await expect(period.getByRole('radio', { name: '7 days', exact: true })).toBeChecked();
  await period.getByRole('radio', { name: 'Custom', exact: true }).click();
  await custom.getByRole('button', { name: 'Last 14 days', exact: true }).click();
  await expect(custom).toContainText('14 days');
  const today = new Date().toISOString().slice(0, 10);
  const start = new Date(Date.now() - 13 * 86400000).toISOString().slice(0, 10);
  const ranged = page.waitForResponse(
    (r) => r.url().includes(`/analytics?from=${start}&to=${today}`) && r.ok(),
  );
  await custom.getByRole('button', { name: 'Apply', exact: true }).click();
  await ranged;
  await expect(custom).toHaveCount(0);
  await expect(period.getByRole('radio', { checked: true })).toContainText('–');
  await expect(page.locator('.analytics-trend .chart-timezone')).toContainText('UTC');
  await page.getByRole('button', { name: 'Journey: Cart recovery', exact: true }).click();
  await page.getByRole('combobox', { name: 'Find a journey…' }).press('Escape');
  await expect(page.getByRole('dialog', { name: 'Journey', exact: true })).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Journey: Cart recovery', exact: true }),
  ).toBeFocused();
  await expect(page.getByText('All-time totals', { exact: true })).toBeVisible();
  // A comparison row leads to that journey's own analytics rather than its builder.
  await page
    .getByRole('table')
    .last()
    .getByRole('button', { name: 'Cart recovery', exact: true })
    .click();
  await expect(page).toHaveURL(/\/journeys\/[0-9a-f-]+\?tab=analytics$/);
});
