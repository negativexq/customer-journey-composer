import { chromium, expect } from '@playwright/test';
import { mkdir, stat, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { seedProductDemo, localDemoURL } from './seed-product-demo.mjs';

const base = localDemoURL(process.env.CJC_WEB_URL || 'http://127.0.0.1:5173');
const demo = await seedProductDemo();
const directory = 'docs/images';
await mkdir(directory, { recursive: true });
const browser = await chromium.launch();
const viewport = { width: 1600, height: 1050 };
const page = await browser.newPage({
  baseURL: base,
  viewport,
  deviceScaleFactor: 2,
  colorScheme: 'light',
  reducedMotion: 'reduce',
  locale: 'en-US',
});
const manifest = {
  captured_at: new Date().toISOString(),
  source_commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
  source_dirty: !!execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim(),
  synthetic_data: true,
  workspace: 'Northstar Commerce',
  story: 'One newly authored Abandoned Cart Recovery journey',
  live_email_provider: 'Local Mailpit SMTP',
  live_cadence: demo.liveCadence,
  draft_cadence: demo.draftCadence,
  viewport,
  device_scale_factor: 2,
  live_showcase_provider: 'mock_sms',
  email_previews: 'isolated simulation',
  screenshots: [],
};
async function settle() {
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0, { timeout: 15000 });
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  });
}
async function open(path) {
  await page.goto(path);
  await page.locator('.app-shell').waitFor();
  await settle();
}
async function shot(name) {
  await settle();
  const file = `${directory}/${name}.png`;
  await page.screenshot({ path: file, animations: 'disabled', caret: 'hide' });
  manifest.screenshots.push({
    file,
    width: page.viewportSize().width * 2,
    height: page.viewportSize().height * 2,
    bytes: (await stat(file)).size,
  });
  console.log('Captured ' + name);
}
try {
  await page.goto('/');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.locator('.app-shell').waitFor();
  await expect
    .poll(
      async () => {
        const summary = await (await page.request.get('/api/v1/overview')).json();
        return (
          summary.metrics.find((m) => m.dimension === 'execution' && m.metric === 'entries')
            ?.value || 0
        );
      },
      { timeout: 15000 },
    )
    .toBeGreaterThanOrEqual(14);
  const cleanJourneys = await (await page.request.get('/api/v1/journeys?limit=200')).json();
  expect(cleanJourneys).toHaveLength(1);
  expect(cleanJourneys[0].name).toBe('Abandoned Cart Recovery');
  const beforeSummary = await (await page.request.get('/api/v1/overview')).json();
  manifest.live_entries_before = beforeSummary.metrics.find(
    (m) => m.dimension === 'execution' && m.metric === 'entries',
  )?.value;
  await open('/');
  await shot('overview');
  await open('/journeys');
  await page.setViewportSize({ width: 1600, height: 760 });
  await shot('journeys');
  await page.setViewportSize(viewport);
  await open('/journeys/' + demo.journeyId);
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.locator('.canvas .vue-flow__node[data-id="purchase_wait"]').click();
  await expect(page.getByRole('complementary', { name: 'Step settings' })).toBeVisible();
  await page.getByRole('button', { name: 'Focus selected step', exact: true }).click();
  await shot('journey-builder');
  await open(`/journeys/${demo.journeyId}?tab=preview-test&view=test-lab`);
  await page
    .getByLabel('Saved scenario')
    .and(page.locator('select'))
    .selectOption({ label: 'Email and SMS · no purchase' });
  await page.getByLabel('Use current draft for regression', { exact: false }).check();
  const creating = page.waitForResponse(
    (response) =>
      response.url().endsWith('/api/v1/test-runs') &&
      response.request().method() === 'POST' &&
      response.ok(),
  );
  await page.getByRole('button', { name: 'Create test run', exact: true }).click();
  const inboxRunId = (await (await creating).json()).id;
  await page.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(page.getByTestId('test-result')).toHaveText('PASS');
  await shot('test-lab');
  await open('/test-inbox');
  await page.getByLabel('Test run', { exact: true }).selectOption(inboxRunId);
  await expect(page.locator('.inbox-row')).toHaveCount(2);
  await page.locator('.inbox-row').filter({ hasText: 'EMAIL' }).click();
  await expect(
    page.frameLocator('iframe[title="Test email preview"]').getByText(/Made for your everyday/),
  ).toBeVisible();
  await shot('test-inbox-email');
  await page.locator('.inbox-row').filter({ hasText: 'SMS' }).click();
  await expect(page.locator('.sms-phone')).toContainText('COMEBACK10');
  await shot('test-inbox-sms');
  await open(`/journeys/${demo.journeyId}?tab=live-monitor&execution=${demo.monitorExecutionId}`);
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText(
    demo.monitorCustomerId,
  );
  await shot('live-monitor');
  await page.getByRole('button', { name: 'Close customer journey', exact: true }).click();
  await page.getByRole('combobox', { name: 'Step', exact: true }).click();
  await page.getByRole('option', { name: 'Your cart is waiting', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Step breakdown' })).toBeVisible();
  await shot('journey-observability/node-breakdown');
  await open(`/journeys/${demo.journeyId}?tab=live-monitor&execution=${demo.monitorExecutionId}`);
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText(
    demo.monitorCustomerId,
  );
  await shot('journey-observability/customer-path');
  await open(`/journeys/${demo.journeyId}?tab=analytics`);
  await expect(page.getByRole('heading', { name: 'Journey outcomes', exact: true })).toBeVisible();
  await shot('journey-observability/analytics');
  await open('/analytics');
  await page.getByRole('button', { name: 'Trend period: Last 30 days', exact: true }).click();
  await page.getByRole('option', { name: /Last 7 days/ }).click();
  await shot('analytics');
  await open(`/customers?customer=${demo.customerId}`);
  await expect(page.getByLabel('Customer ID', { exact: true })).toHaveValue(demo.customerId);
  await shot('customers');
  await open(`/journeys/${demo.journeyId}`);
  await page.getByRole('button', { name: 'Publish', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Review before publishing' })).toBeVisible();
  await page
    .getByText('Checking workflow, test evidence and provider configuration…')
    .waitFor({ state: 'hidden' });
  await page
    .getByRole('dialog', { name: 'Review before publishing' })
    .getByRole('checkbox', { name: 'I reviewed this version', exact: false })
    .check();
  await shot('publish-review');
  const afterSummary = await (await page.request.get('/api/v1/overview')).json();
  manifest.live_entries_after = afterSummary.metrics.find(
    (m) => m.dimension === 'execution' && m.metric === 'entries',
  )?.value;
  expect(manifest.live_entries_after).toBe(manifest.live_entries_before);
  await writeFile(directory + '/manifest.json', JSON.stringify(manifest, null, 2) + '\n');
  console.log(
    `Saved ${manifest.screenshots.length} current application screenshots at native 2× scale.`,
  );
} finally {
  await browser.close();
}
