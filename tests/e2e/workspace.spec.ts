import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('builder panels preserve the graph and duration controls persist typed seconds', async ({
  page,
}) => {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Workspace editor',
        workspace: 'Design workspace',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const j = await (
    await page.request.post('/api/v1/templates/abandoned-cart/create-journey', {
      headers: { 'X-CSRF-Token': identity.csrf_token },
      data: {},
    })
  ).json();
  await page.goto('/journeys/' + j.id);
  await expect(page.getByText('Editing draft', { exact: true })).toBeVisible();
  await expect(page.getByRole('complementary', { name: 'Step settings' })).toBeHidden();
  await expect(page.getByRole('complementary', { name: 'Journey inspector' })).toBeHidden();
  await expect(
    page.getByRole('button', { name: 'Toggle step settings', exact: true }),
  ).toBeDisabled();
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.locator('.vue-flow__node').filter({ hasText: 'Wait 2 hours' }).click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await expect(settings.getByLabel('Delay unit')).toHaveText('Hours');
  await settings.getByLabel('Wait for', { exact: true }).fill('3');
  const libraryButton = page.getByRole('button', { name: 'Toggle step library', exact: true });
  if ((await libraryButton.getAttribute('aria-expanded')) === 'false') await libraryButton.click();
  await libraryButton.click();
  await expect(page.getByRole('complementary', { name: 'Step library' })).toBeHidden();
  await expect(page.locator('.vue-flow__node')).toHaveCount(8);
  await page.getByRole('button', { name: 'Toggle step library', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Step library' })).toBeVisible();
  await page.getByRole('button', { name: 'Toggle step settings', exact: true }).click();
  await expect(settings).toBeHidden();
  await page.getByRole('button', { name: 'Toggle step settings', exact: true }).click();
  await expect(settings.getByLabel('Wait for', { exact: true })).toHaveValue('3');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  const saved = await (await page.request.get('/api/v1/journeys/' + j.id)).json();
  expect(
    saved.draft.nodes.find((n: { id: string }) => n.id === 'delay').config.duration_seconds,
  ).toBe(10800);
  await page.reload();
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.locator('.vue-flow__node').filter({ hasText: 'Wait 2 hours' }).click();
  await expect(settings.getByLabel('Wait for', { exact: true })).toHaveValue('3');
});
