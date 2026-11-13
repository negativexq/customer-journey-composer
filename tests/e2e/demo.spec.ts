import { expect, test } from '@playwright/test';
test('local demo credentials are prefilled and sign in without typing', async ({ page }) => {
  await page.goto('/');
  await expect(
    page.getByText('Local demo account is ready. Click Sign in to explore.'),
  ).toBeVisible();
  expect(await page.getByLabel('Email', { exact: true }).inputValue()).toBe('demo@example.invalid');
  // Check presence without writing the generated password into test output.
  expect(
    (await page.getByLabel('Password', { exact: true }).inputValue()).length >= 16,
  ).toBeTruthy();
  const signingIn = page.waitForResponse(
    (response) =>
      response.url().endsWith('/api/v1/auth/login') &&
      response.request().method() === 'POST' &&
      response.ok(),
  );
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const identity = await (await signingIn).json();
  const greeting = page.getByRole('heading', { name: /Good (morning|afternoon|evening),/ });
  await expect(greeting).toBeVisible();
  await expect(greeting).toContainText(identity.name.split(' ')[0]);
});
