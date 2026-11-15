import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { choose } from './helpers/select';

test('settings are split into linkable sections with readable administration', async ({ page }) => {
  await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Workspace Owner',
      workspace: 'Settings',
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    },
  });
  const sections = page.getByRole('navigation', { name: 'Settings sections' });
  const open = (name: string) => sections.getByRole('button', { name, exact: true }).click();

  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Messaging', exact: true })).toBeVisible();
  await expect(sections.getByRole('button', { name: 'Messaging', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
  // One section at a time: nothing from the others is on the page.
  await expect(page.getByRole('heading', { name: 'Team', exact: true })).toHaveCount(0);
  await page.getByLabel('Maximum messages per customer', { exact: true }).fill('5');
  await page.getByRole('button', { name: 'Save messaging policy', exact: true }).click();
  await expect(page.getByTestId('toast')).toContainText('Messaging policy saved');

  await open('API keys');
  await expect(page).toHaveURL(/\/settings\?section=api-keys$/);
  await expect(page.getByText('No API keys yet', { exact: true })).toBeVisible();
  await page.getByLabel('Key name', { exact: true }).fill('Storefront');
  await page.getByRole('button', { name: 'Create API key', exact: true }).click();
  await expect(page.getByLabel('New API key', { exact: true })).not.toHaveValue('');
  await page.getByRole('button', { name: 'Hide key', exact: true }).click();
  const keyRow = page.getByRole('listitem').filter({ hasText: 'Storefront' });
  await expect(keyRow).toContainText('Active');
  await keyRow.getByRole('button', { name: 'Revoke', exact: true }).click();
  const confirm = page.getByRole('dialog', { name: 'Revoke this API key?' });
  await expect(confirm).toContainText('stops working immediately');
  await confirm.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(keyRow).toContainText('Active');
  await keyRow.getByRole('button', { name: 'Revoke', exact: true }).click();
  await confirm.getByRole('button', { name: 'Revoke key', exact: true }).click();
  await expect(keyRow).toContainText('Revoked');
  await expect(keyRow.getByRole('button', { name: 'Revoke', exact: true })).toHaveCount(0);

  await open('Team');
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Team', exact: true })).toBeVisible();
  await expect(page.getByLabel('Role for Workspace Owner')).toHaveText('Admin');
  await expect(page.getByLabel('User email', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Add member', exact: true }).click();
  await page.getByLabel('User name', { exact: true }).fill('Ada Editor');
  await page
    .getByLabel('User email', { exact: true })
    .fill(randomBytes(8).toString('hex') + '@example.invalid');
  await page.getByLabel('Initial password', { exact: true }).fill(randomBytes(12).toString('hex'));
  await choose(page.getByLabel('New user role', { exact: true }), 'Editor');
  await page.getByRole('button', { name: 'Create user', exact: true }).click();
  await expect(page.getByLabel('Role for Ada Editor')).toHaveText('Editor');
  await choose(page.getByLabel('Role for Ada Editor'), 'Viewer');
  await expect(
    page.getByTestId('toast').filter({ hasText: 'Ada Editor is now Viewer' }),
  ).toBeVisible();

  await open('Audit log');
  const log = page.locator('.audit-list');
  await expect(log).toContainText('Member role changed');
  await expect(log).toContainText('Member added');
  await expect(log).toContainText('API key revoked');
  await expect(log.getByRole('listitem').first()).toContainText('Workspace Owner');
  await expect(log).not.toContainText('T00:');

  await open('System');
  await expect(page.getByText('No failed jobs or undelivered messages')).toBeVisible();
  await expect(page.getByText('Database connections', { exact: true })).toBeVisible();
  await expect(page.locator('pre')).toBeHidden();

  await page.goBack();
  await expect(page.getByRole('heading', { name: 'Audit log', exact: true })).toBeVisible();
});
