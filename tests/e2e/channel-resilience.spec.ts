import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { choose } from './helpers/select';

test('a refused key holds the SMS channel, rings the bell and costs no attempt', async ({
  page,
}, info) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Channel admin',
        workspace: 'Channels',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const phone = '+90555' + String(1000000 + Math.floor(Math.random() * 8999999));
  expect(
    (
      await page.request.put('/api/v1/settings/messaging', {
        headers,
        data: { frequency_limit: 3, frequency_window_seconds: 86400, test_allowlist: [phone] },
      })
    ).ok(),
  ).toBeTruthy();

  await page.goto('/settings');
  const sms = page.locator('.channel-row').filter({ hasText: 'SMS' });
  await expect(sms).toContainText('Sending');
  await choose(sms.getByLabel('Simulate a provider problem'), 'Refused key (401)');
  await sms.getByLabel('For (minutes)').fill('5');
  await sms.getByRole('button', { name: 'Save SMS channel' }).click();
  await expect(sms).toContainText('Simulating refused key (401)');

  const sent = await page.request.post('/api/v1/messages/test-send', {
    headers: { ...headers, 'Idempotency-Key': randomBytes(16).toString('hex') },
    data: {
      recipient: phone,
      preview: {
        channel: 'sms',
        config: { text: 'Use {{ offer.code }} today', discount_code: 'SEPET15' },
      },
    },
  });
  expect(sent.ok()).toBeTruthy();
  const delivery = (await sent.json()).id;

  await expect(sms).toContainText('Credentials refused', { timeout: 15000 });
  await expect(sms).toContainText('Held (429/401)');
  const job = await (await page.request.get(`/api/v1/deliveries/${delivery}`)).json();
  expect(job.status).toBe('SCHEDULED');

  if (info.project.name !== 'desktop') return;
  const bell = page.getByRole('button', { name: /^Notifications/ });
  await expect(bell).toHaveAccessibleName('Notifications, 1 unread', { timeout: 20000 });
  await bell.click();
  const panel = page.getByRole('region', { name: 'Notifications' });
  await expect(panel).toContainText('SMS provider refused the credentials');
  await expect(bell).toHaveAccessibleName('Notifications');
  await panel.getByRole('button', { name: /SMS provider refused/ }).click();
  await expect(page).toHaveURL(/\/settings/);
});

test('an SMS is measured with its code and tracked link as sent', async ({ page }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Code editor',
        workspace: 'Codes',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const preview = await (
    await page.request.post('/api/v1/messages/preview', {
      headers,
      data: {
        channel: 'sms',
        config: {
          text: 'Hi, use {{ offer.code }} at https://shop.example/cart',
          discount_code: 'SEPET15',
        },
      },
    })
  ).json();
  expect(preview.text).toContain('SEPET15');
  expect(preview.longest_case.tracked_links).toBe(1);
  expect(preview.longest_case.units).toBeGreaterThan(preview.units - 20);
});
