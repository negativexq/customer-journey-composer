import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('theme follows the system until someone pins light or dark', async ({ page }) => {
  await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'Night Owl',
      workspace: 'Themes',
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    },
  });
  const surface = () =>
    page.evaluate(() => {
      const probe = document.createElement('i');
      probe.style.color = 'var(--surface)';
      document.body.append(probe);
      const color = getComputedStyle(probe).color;
      probe.remove();
      return color;
    });
  const html = page.locator('html');

  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(html).not.toHaveAttribute('data-theme');
  const dark = await surface();
  await page.emulateMedia({ colorScheme: 'light' });
  const light = await surface();
  expect(light).toBe('rgb(255, 255, 255)');
  expect(dark).not.toBe(light);

  await page.getByRole('button', { name: 'Theme: Match system. Switch to Light' }).click();
  await expect(html).toHaveAttribute('data-theme', 'light');
  await page.getByRole('button', { name: 'Theme: Light. Switch to Dark' }).click();
  await expect(html).toHaveAttribute('data-theme', 'dark');
  // A pinned theme wins over the system and survives a reload.
  expect(await surface()).toBe(dark);
  await page.reload();
  await expect(html).toHaveAttribute('data-theme', 'dark');
  await expect(
    page.getByRole('button', { name: 'Theme: Dark. Switch to Match system' }),
  ).toBeVisible();
  expect(await surface()).toBe(dark);

  await page.getByRole('button', { name: /^Theme: Dark/ }).click();
  await expect(html).not.toHaveAttribute('data-theme');
  expect(await surface()).toBe(light);
});
