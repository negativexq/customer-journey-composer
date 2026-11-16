import { chromium, expect } from '@playwright/test';
import { mkdir, stat, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { seedProductDemo, localDemoURL } from './seed-product-demo.mjs';

const base = localDemoURL(process.env.CJC_WEB_URL || 'http://127.0.0.1:5173');
const demo = await seedProductDemo();
const directory = 'docs/images';
await mkdir(directory, { recursive: true });
const browser = await chromium.launch();
const viewport = { width: 1280, height: 800 };
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
// Bring a section to the top of the window, so a shot shows it rather than the page header.
async function reveal(locator) {
  await locator.evaluate((el) => {
    el.scrollIntoView({ block: 'start' });
    // A little air above, without a sliver of the cards above it.
    let box = el.parentElement;
    while (box && box.scrollHeight <= box.clientHeight) box = box.parentElement;
    (box || document.scrollingElement).scrollBy(0, -14);
  });
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
  await page.setViewportSize({ width: 1280, height: 560 });
  await shot('journeys');
  await page.setViewportSize(viewport);
  await open('/journeys/' + demo.journeyId);
  // Focus pans a step that sits under the canvas toolbar into view before it is clicked.
  const purchaseWait = page.locator('.canvas .vue-flow__node[data-id="purchase_wait"]');
  await purchaseWait.focus();
  await settle();
  await purchaseWait.click();
  await expect(page.getByRole('complementary', { name: 'Step settings' })).toBeVisible();
  // Close enough to read: centre the selected step rather than fit all seven.
  await page.getByLabel('More canvas actions').click();
  await page.getByRole('button', { name: 'Focus selected step', exact: true }).click();
  await settle();
  // Selecting a step can scroll the page; measure with the header in view.
  await page.locator('.canvas').evaluate((el) => {
    for (let box = el; box; box = box.parentElement) box.scrollTop = 0;
    for (const box of el.querySelectorAll('*')) if (box.scrollTop) box.scrollTop = 0;
  });
  // Pan so the selected wait sits near the top and both of its branches fit across the
  // canvas, whatever height the canvas has. The drag starts on empty canvas left of the step.
  const pane = await page.locator('.canvas .vue-flow__pane').boundingBox();
  const wait = await page.locator('.canvas .vue-flow__node[data-id="purchase_wait"]').boundingBox();
  const left = await page.locator('.canvas .vue-flow__node[data-id="converted"]').boundingBox();
  const right = await page.locator('.canvas .vue-flow__node[data-id="sms"]').boundingBox();
  const dx = pane.x + pane.width / 2 - (left.x + right.x + right.width) / 2;
  const dy = pane.y + 24 - wait.y;
  const from = { x: wait.x - 40, y: wait.y + wait.height / 2 };
  const target = await page.evaluate(
    ({ x, y }) => document.elementFromPoint(x, y)?.classList.contains('vue-flow__pane'),
    from,
  );
  if (!target) throw new Error('The builder pan must start on empty canvas.');
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.mouse.move(from.x + dx, from.y + dy, { steps: 8 });
  await page.mouse.up();
  await page.mouse.move(5, 5);
  await shot('journey-builder');
  await open(`/journeys/${demo.journeyId}?tab=preview-test&view=test-lab`);
  await page.getByRole('combobox', { name: 'Saved scenario' }).click();
  await page.getByRole('option', { name: 'Email and SMS · no purchase' }).click();
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
  // Runs are listed newest first, so the first match is the one just created.
  void inboxRunId;
  await page.getByRole('combobox', { name: 'Test run', exact: true }).click();
  await page
    .getByRole('option', { name: /Email and SMS · no purchase/ })
    .first()
    .click();
  await expect(page.locator('.inbox-row')).toHaveCount(2);
  await page.locator('.inbox-row').filter({ hasText: 'EMAIL' }).click();
  await expect(
    page.frameLocator('iframe[title="Test email preview"]').getByText(/Made for your everyday/),
  ).toBeVisible();
  // Taller, so the email body shows under the message details.
  await page.setViewportSize({ width: 1280, height: 1180 });
  await shot('test-inbox-email');
  await page.setViewportSize(viewport);
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
  await reveal(page.locator('.monitor-sections'));
  await shot('journey-observability/node-breakdown');
  await open(`/journeys/${demo.journeyId}?tab=live-monitor&execution=${demo.monitorExecutionId}`);
  await expect(page.getByRole('region', { name: 'Execution details' })).toContainText(
    demo.monitorCustomerId,
  );
  await reveal(page.locator('.monitor-sections'));
  await shot('journey-observability/customer-path');
  await open(`/journeys/${demo.journeyId}?tab=analytics`);
  await expect(page.getByRole('heading', { name: 'Journey outcomes', exact: true })).toBeVisible();
  await shot('journey-observability/analytics');
  await open('/analytics');
  await page.getByRole('button', { name: 'Trend period: Last 30 days', exact: true }).click();
  await page.getByRole('option', { name: /Last 7 days/ }).click();
  await shot('analytics');
  await open(`/customers?customer=${demo.customerId}`);
  await expect(page.getByRole('heading', { level: 2, name: 'Alex Morgan' })).toBeVisible();
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
  // Tall enough for the whole review, so the dialog does not open scrolled.
  await page.setViewportSize({ width: 1280, height: 1080 });
  await shot('publish-review');
  await page.setViewportSize(viewport);
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
