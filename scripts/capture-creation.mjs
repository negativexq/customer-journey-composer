import { chromium, expect } from '@playwright/test';
import { randomBytes } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdir, stat, writeFile } from 'node:fs/promises';
import { showcaseRecipe, showcaseName, showcaseDescription } from './showcase-recipe.mjs';
import { localDemoURL } from './seed-product-demo.mjs';

const base = localDemoURL(process.env.CJC_WEB_URL || 'http://127.0.0.1:5173');
const directory = 'docs/images/creation';
const source = 'scripts/showcase-recipe.mjs';
const live = process.env.CJC_LIVE_AI === '1';
const recipe = showcaseRecipe();
await mkdir(directory, { recursive: true });
await mkdir('.tmp', { recursive: true });
const viewport = { width: 1600, height: 1050 };
const manifest = {
  captured_at: new Date().toISOString(),
  source_commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
  source_dirty: !!execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim(),
  viewport,
  device_scale_factor: 2,
  synthetic_data: true,
  authored_workflow: live
    ? 'Catalog and reviewed contract from ' + source + '; workflow is the unedited live_proposal'
    : source,
  workspace: 'Northstar Commerce',
  generation: live
    ? 'Live OpenAI generation through the application API; no response interception'
    : 'Local SSE replay of the newly authored showcase proposal; no live provider call',
  presentation_metadata: live
    ? 'Real provider proposal for the displayed brief; operational demo is separately authored'
    : 'One shared human-authored journey recipe across the galleries',
  screenshots: [],
};
const proposal = {
  schema_version: 'composer.v5',
  action: 'generate',
  name: showcaseName,
  description: showcaseDescription,
  workflow: recipe.workflow,
  explanation:
    'Wait one hour, send a personal email, then wait up to 24 hours for the same cart to be purchased. If no purchase arrives, offer 10% off by SMS. Channel permissions are enforced at delivery. A matching purchase ends the journey immediately.',
  warnings: [
    'The storefront URL is fictional. Live gallery activity uses local Mailpit and mock SMS.',
  ],
  diff: {
    metadata_changed: true,
    edges_changed: true,
    exit_changed: true,
    reentry_changed: true,
    added_nodes: recipe.workflow.nodes.map((n) => n.id),
    removed_nodes: [],
    updated_nodes: [],
    changed_conditions: [],
    changed_messages: [],
  },
};
const brief =
  'Recover abandoned EUR carts worth at least €50. After one hour, send a personal email. Wait up to 24 hours for the shopper to purchase the same cart. If they do, stop immediately as converted. Otherwise offer 10% off by SMS and complete. Respect channel consent and prevent re-entry for seven days.';

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
if (!live) await new Promise((resolve) => replay.listen(0, '127.0.0.1', resolve));
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
        name: 'Alex Morgan',
        workspace: 'Northstar Commerce',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const catalog = await page.request.put('/api/v1/data-catalog', {
    headers,
    data: { entries: recipe.catalog },
  });
  if (!catalog.ok()) throw new Error('Could not prepare the fictional workspace catalog.');
  if (!live)
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
  manifest.journey_id = id;
  const record = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
  const saved = await page.request.put(`/api/v1/journeys/${id}/composition`, {
    headers,
    data: {
      name: proposal.name,
      description: proposal.description,
      revision: record.journey.revision,
      state: {
        ...record.state,
        brief: brief,
        provider: 'openai',
        behavior_checks: recipe.contract,
      },
    },
  });
  if (!saved.ok()) throw new Error('Could not save the new brief and scoped contract.');
  await page.reload();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(brief);
  await shot('brief');
  await page
    .getByRole('heading', { name: 'What must be true?', exact: true })
    .evaluate((element) => element.scrollIntoView({ block: 'start' }));
  await shot('contract');
  if (live)
    await page.evaluate(() => {
      const originalFetch = window.fetch.bind(window);
      window.fetch = async (...args) => {
        const response = await originalFetch(...args);
        if (response.url.endsWith('/journeys/generate/stream'))
          window.__captureStream = response.clone().text();
        return response;
      };
    });
  const started = Date.now();
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect(page.getByText('Waiting for the AI provider', { exact: false })).toBeVisible();
  await shot('generating');
  if (!live) finish();
  if (live) {
    const body = await page.evaluate(async () => {
      if (!window.__captureStream) throw new Error('Live generation stream was not observed');
      return await window.__captureStream;
    });
    const events = body
      .split('\n')
      .filter((line) => line.startsWith('data: '))
      .map((line) => JSON.parse(line.slice(6)));
    await writeFile('.tmp/live-creation-events.json', JSON.stringify(events, null, 2));
    const ready = events.find((event) => event.type === 'ready');
    if (!ready?.proposal) throw new Error('No live proposal returned: ' + JSON.stringify(events));
    Object.assign(proposal, ready.proposal);
    manifest.duration_ms = Date.now() - started;
    manifest.live_proposal = ready.proposal;
    await writeFile(directory + '/manifest.json', JSON.stringify(manifest, null, 2) + '\n');
    console.log(
      JSON.stringify({ duration_ms: manifest.duration_ms, generation: ready.proposal.generation }),
    );
  }
  await expect(page.getByRole('heading', { name: 'Review your journey', exact: true })).toBeVisible(
    { timeout: 15000 },
  );
  await expect(page.locator('.creation-save')).toHaveText('All changes saved');
  await expect(page.getByRole('region', { name: 'Reviewed behavior expectations' })).toContainText(
    'Purchase before the first reminder',
  );
  await shot('review');
  await page
    .getByRole('region', { name: 'Journey plan', exact: true })
    .evaluate((element) => element.scrollIntoView({ block: 'start' }));
  await shot('review-diagram');
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.locator('.canvas .vue-flow__node')).toHaveCount(proposal.workflow.nodes.length);
  await shot('canvas-start');
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  const waitNode = proposal.workflow.nodes.find((node) => node.type === 'wait_event');
  if (!waitNode) throw new Error('Live workflow has no wait-for-event node to inspect');
  await page.locator(`.canvas .vue-flow__node[data-id="${waitNode.id}"]`).click();
  await page.getByRole('button', { name: 'Focus selected step', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Step settings' })).toBeVisible();
  await shot('canvas-focus');
  expect(await (await page.request.get(`/api/v1/journeys/${id}/versions`)).json()).toHaveLength(0);
  expect(await (await page.request.get('/api/v1/deliveries')).json()).toHaveLength(0);
  await writeFile(directory + '/manifest.json', JSON.stringify(manifest, null, 2) + '\n');
  console.log(
    live
      ? 'Captured eight creation screens from live AI generation; no publication.'
      : 'Captured eight creation screens with local replay; no provider call or publication.',
  );
} finally {
  finish?.();
  await browser.close();
  if (!live) {
    replay.closeAllConnections();
    await new Promise((resolve) => replay.close(resolve));
  }
}
