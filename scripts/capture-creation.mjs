import { chromium, expect } from '@playwright/test';
import { randomBytes } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises';
import { localDemoURL } from './seed-product-demo.mjs';

const base = localDemoURL(process.env.CJC_WEB_URL || 'http://127.0.0.1:5173');
const directory = 'docs/images/creation';
const source = 'docs/validation/scoped-contract-live-2026-11-13/result.json';
const recorded = JSON.parse(await readFile(source, 'utf8')).journeys.find((j) => j.id === 'stock');
if (!recorded?.proposal || recorded.cases.some((c) => c.result !== 'PASS'))
  throw new Error('Expected the recorded passing stock workflow.');
await mkdir(directory, { recursive: true });
const viewport = { width: 1600, height: 1050 };
const manifest = {
  captured_at: new Date().toISOString(),
  source_commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
  viewport,
  device_scale_factor: 2,
  synthetic_data: true,
  recorded_workflow: source + '#stock',
  generation: 'Local SSE replay of the recorded proposal; no live provider call',
  presentation_metadata:
    'Human-authored demo journey name/description; graph and contract from the recorded run',
  screenshots: [],
};
const proposal = {
  ...recorded.proposal,
  name: 'Back in stock',
  description: 'Notify shoppers when the product they want becomes available.',
};
let finish;
const replay = createServer((request, response) => {
  request.resume();
  response.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Access-Control-Allow-Origin': base,
    'Access-Control-Allow-Credentials': 'true',
    'Cache-Control': 'no-store',
  });
  response.write(
    'data: ' + JSON.stringify({ type: 'progress', stage: 'generating', attempt: 1 }) + '\n\n',
  );
  finish = () => {
    if (!response.writableEnded)
      response.end('data: ' + JSON.stringify({ type: 'ready', proposal }) + '\n\n');
  };
});
await new Promise((resolve) => replay.listen(0, '127.0.0.1', resolve));
const browser = await chromium.launch();
const page = await browser.newPage({
  baseURL: base,
  viewport,
  deviceScaleFactor: 2,
  colorScheme: 'light',
  reducedMotion: 'reduce',
  locale: 'en-US',
});
async function shot(name) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
  const file = `${directory}/${name}.png`;
  await page.screenshot({ path: file, animations: 'disabled', caret: 'hide' });
  manifest.screenshots.push({ file, width: 3200, height: 2100, bytes: (await stat(file)).size });
  console.log('Captured creation/' + name);
}
try {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Product showcase',
        workspace: 'Product showcase',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const catalog = await page.request.put('/api/v1/data-catalog', {
    headers,
    data: { entries: recorded.catalog },
  });
  if (!catalog.ok()) throw new Error('Could not prepare the fictional workspace catalog.');
  await page.route('**/api/v1/ai/**', (route) => {
    if (route.request().url().endsWith('/journeys/generate/stream'))
      return route.continue({ url: `http://127.0.0.1:${replay.address().port}/generate` });
    return route.abort();
  });
  await page.goto('/journeys/new');
  await page.getByLabel('Journey name', { exact: true }).fill(proposal.name);
  await page.getByLabel('Description', { exact: false }).fill(proposal.description);
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await shot('method');
  await page.getByRole('button', { name: 'Create with AI', exact: false }).click();
  await expect(page.getByLabel('Your brief', { exact: true })).toBeVisible();
  await expect(page.locator('.creation-save')).toHaveText('All changes saved');
  const id = page.url().split('/').at(-1);
  const record = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
  const saved = await page.request.put(`/api/v1/journeys/${id}/composition`, {
    headers,
    data: {
      name: proposal.name,
      description: proposal.description,
      revision: record.journey.revision,
      state: {
        ...record.state,
        brief: recorded.brief,
        provider: 'openai',
        behavior_checks: recorded.behavior_checks,
      },
    },
  });
  if (!saved.ok()) throw new Error('Could not save the recorded brief and scoped contract.');
  await page.reload();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(recorded.brief);
  await shot('brief');
  await page
    .getByRole('heading', { name: 'What must be true?', exact: true })
    .evaluate((element) => element.scrollIntoView({ block: 'start' }));
  await shot('contract');
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect(page.getByText('Waiting for the AI provider', { exact: false })).toBeVisible();
  await shot('generating');
  finish();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await expect(page.locator('.creation-save')).toHaveText('All changes saved');
  await expect(page.getByRole('region', { name: 'Reviewed behavior expectations' })).toContainText(
    'Already in stock at entry',
  );
  await shot('review');
  await page
    .getByRole('region', { name: 'Journey plan', exact: true })
    .evaluate((element) => element.scrollIntoView({ block: 'start' }));
  await shot('review-diagram');
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.locator('.canvas .vue-flow__node')).toHaveCount(
    recorded.proposal.workflow.nodes.length,
  );
  await shot('canvas-start');
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.locator('.canvas .vue-flow__node[data-id="wait_for_stock"]').click();
  await page.getByRole('button', { name: 'Focus selected step', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Step settings' })).toBeVisible();
  await shot('canvas-focus');
  expect(await (await page.request.get(`/api/v1/journeys/${id}/versions`)).json()).toHaveLength(0);
  expect(await (await page.request.get('/api/v1/deliveries')).json()).toHaveLength(0);
  await writeFile(directory + '/manifest.json', JSON.stringify(manifest, null, 2) + '\n');
  console.log(
    'Captured eight creation screens with local replay; no provider call or publication.',
  );
} finally {
  finish?.();
  await browser.close();
  replay.closeAllConnections();
  await new Promise((resolve) => replay.close(resolve));
}
