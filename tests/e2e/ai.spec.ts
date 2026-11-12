import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
const workflow = {
  schema_version: 1,
  nodes: [
    {
      id: 'start',
      type: 'event_trigger',
      label: 'Cart updated',
      position: { x: 0, y: 0 },
      config: { event_name: 'cart_updated' },
    },
    {
      id: 'done',
      type: 'exit',
      label: 'Complete',
      position: { x: 0, y: 180 },
      config: { reason: 'completed' },
    },
  ],
  edges: [{ id: 'edge', source: 'start', target: 'done', port: 'started' }],
};
const fixture = {
  schema_version: 'composer.v2',
  generation: {
    attempts: [
      {
        provider: 'claude',
        provider_succeeded: true,
        accepted: true,
        requested_model: 'contract-fixture',
        model: 'contract-fixture',
        duration_ms: 1,
        usage: {
          input_tokens: 12,
          output_tokens: 4,
          reasoning_tokens: 0,
          cached_input_tokens: 0,
          cache_creation_tokens: null,
        },
      },
    ],
    usage: {
      input_tokens: 12,
      output_tokens: 4,
      reasoning_tokens: 0,
      cached_input_tokens: 0,
      cache_creation_tokens: null,
    },
  },
  action: 'generate',
  name: 'AI reviewed journey',
  description: 'A reviewed proposal',
  workflow,
  explanation: 'Starts with a cart event and completes.',
  warnings: ['Confirm entry and exit settings'],
  diff: {
    metadata_changed: true,
    added_nodes: ['start', 'done'],
    removed_nodes: [],
    updated_nodes: [],
    changed_conditions: [],
    changed_messages: [],
    edges_changed: true,
    exit_changed: false,
    reentry_changed: false,
  },
};
async function setup(page: import('@playwright/test').Page) {
  const signup = await page.request.post('/api/v1/auth/register', {
    data: {
      name: 'AI editor',
      workspace: 'AI review workspace',
      email: `${randomBytes(12).toString('hex')}@example.invalid`,
      password: randomBytes(24).toString('hex'),
    },
  });
  const user = await signup.json();
  await page.request.put('/api/v1/data-catalog', {
    headers: { 'X-CSRF-Token': user.csrf_token },
    data: {
      entries: [
        {
          kind: 'event',
          event_name: 'cart_updated',
          label: 'Cart updated',
          data_type: 'event',
          aliases: [],
          enabled: true,
        },
      ],
    },
  });
  const created = await page.request.post('/api/v1/journeys', {
    headers: { 'X-CSRF-Token': user.csrf_token },
    data: { name: 'AI draft' },
  });
  const journey = await created.json();
  await page.route('**/api/v1/settings/ai', (route) =>
    route.fulfill({
      json: {
        enabled: true,
        default_provider: 'claude',
        providers: {
          claude: { configured: true, model: 'contract-fixture' },
          openai: { configured: true, model: 'contract-fixture' },
        },
      },
    }),
  );
  await page.route('**/api/v1/ai/journeys/generate/stream', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `data: ${JSON.stringify({ type: 'ready', proposal: fixture })}\n\n`,
    }),
  );
  await page.goto('/');
  await page.getByRole('button', { name: 'AI draft', exact: false }).click();
  return journey;
}
test('AI proposal requires review and applies only after human approval', async ({ page }) => {
  const journey = await setup(page);
  await page
    .getByLabel('What should happen?', { exact: true })
    .fill('Start when a cart is updated.');
  await page.getByRole('button', { name: 'Compose proposal', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Review proposal' })).toBeVisible();
  await expect(page.locator('.ai-proposal')).toContainText('Reported tokens: 12 input / 4 output.');
  expect(
    (await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()).draft.nodes,
  ).toHaveLength(0);
  await expect(page.locator('.vue-flow__node')).toHaveCount(0);
  await page.getByRole('button', { name: 'Discard proposal', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(0);
  await page.getByRole('button', { name: 'Compose proposal', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Apply proposal', exact: true })).toBeDisabled();
  await page.getByLabel('I reviewed the warnings and assumptions').check();
  await page.getByRole('button', { name: 'Apply proposal', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(2);
  await expect
    .poll(async () => {
      const saved = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
      return saved.draft.nodes.length;
    })
    .toBe(2);
  const saved = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(saved.status).toBe('Draft');
  expect(saved.name).toBe('AI reviewed journey');
});
test('AI cannot apply a stale proposal over a newer draft edit', async ({ page }) => {
  await setup(page);
  await page.getByRole('button', { name: 'Compose proposal', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Review proposal' })).toBeVisible();
  await page.getByLabel('I reviewed the warnings and assumptions').check();
  await page.getByLabel('More journey actions').click();
  await page.getByRole('button', { name: 'Journey settings', exact: true }).click();
  await page.getByLabel('Name', { exact: true }).fill('A newer human edit');
  await expect(page.getByRole('button', { name: 'Apply proposal', exact: true })).toBeDisabled();
  await expect(
    page.getByText('Your draft changed after this request.', { exact: false }),
  ).toBeVisible();
  await expect(page.locator('.vue-flow__node')).toHaveCount(0);
});

test('long generation stays visible and cancellation never submits again', async ({ page }) => {
  await setup(page);
  let requests = 0;
  let release!: () => void;
  const waiting = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route('**/api/v1/ai/journeys/generate/stream', async (route) => {
    requests++;
    await waiting;
    await route
      .fulfill({
        contentType: 'text/event-stream',
        body: `data: ${JSON.stringify({ type: 'ready', proposal: fixture })}\n\n`,
      })
      .catch(() => {});
  });
  await page.clock.install();
  await page
    .getByLabel('What should happen?', { exact: true })
    .fill('Keep this brief while waiting');
  await page.getByRole('button', { name: 'Compose proposal', exact: true }).click();
  await expect(page.locator('.canvas-generation')).toContainText('Creating your proposal');
  await page.clock.fastForward(45000);
  await expect(page.locator('.ai-panel .generation-status')).toContainText('0:45 elapsed');
  await expect(page.locator('.canvas-generation')).toContainText('no need to submit again');
  await page
    .locator('.ai-panel')
    .getByRole('button', { name: 'Cancel request', exact: true })
    .click();
  await expect(page.locator('.ai-panel [role="alert"]')).toContainText('Cancellation requested');
  await expect(page.getByLabel('What should happen?', { exact: true })).toHaveValue(
    'Keep this brief while waiting',
  );
  release();
  await expect(page.locator('.canvas-generation')).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Review proposal' })).toHaveCount(0);
  expect(requests).toBe(1);
});

test('server progress and failure are shown without changing the draft', async ({ page }) => {
  await setup(page);
  let requests = 0;
  await page.route('**/api/v1/ai/journeys/generate/stream', (route) => {
    requests++;
    return route.fulfill({
      contentType: 'text/event-stream',
      body: [
        { type: 'progress', stage: 'generating', attempt: 1 },
        { type: 'progress', stage: 'validating', attempt: 1 },
        { type: 'progress', stage: 'repairing', attempt: 1 },
        {
          type: 'error',
          status: 422,
          message: 'The workflow could not be validated. Refine your brief.',
        },
      ]
        .map((event) => `data: ${JSON.stringify(event)}\n\n`)
        .join(''),
    });
  });
  await page.getByRole('button', { name: 'Compose proposal', exact: true }).click();
  await expect(page.locator('.ai-panel [role="alert"]')).toContainText('could not be validated');
  await expect(page.locator('.vue-flow__node')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Compose proposal', exact: true })).toBeEnabled();
  expect(requests).toBe(1);
});

async function startGuided(page: import('@playwright/test').Page, name: string) {
  await setup(page);
  await page.goto('/journeys');
  await page
    .locator('.page-heading')
    .getByRole('button', { name: 'New journey', exact: true })
    .click();
  await expect(page).toHaveURL(/journeys\/new$/);
  await page.getByLabel('Journey name', { exact: true }).fill(name);
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await page.getByRole('button', { name: 'Create with AI', exact: false }).click();
  await expect(
    page.getByRole('heading', { name: 'Describe the journey', exact: true }),
  ).toBeVisible();
  return page.url().split('/').at(-1)!;
}
test('creation is a page and resumes a saved brief without making a model request', async ({
  page,
}) => {
  const id = await startGuided(page, 'Resume this brief');
  let requests = 0;
  page.on('request', (request) => {
    if (request.url().includes('/ai/journeys/')) requests++;
  });
  await page.getByRole('button', { name: 'Trial conversion', exact: false }).click();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(/trial customers/);
  expect(await page.getByLabel('Your brief', { exact: true }).inputValue()).not.toMatch(
    /never publish|do not publish/i,
  );
  await page
    .getByLabel('Your brief', { exact: true })
    .fill('Keep this unfinished brief for tomorrow');
  await page.getByRole('button', { name: '← Journeys', exact: true }).click();
  await page.getByRole('button', { name: 'Resume this brief', exact: true }).click();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(
    'Keep this unfinished brief for tomorrow',
  );
  await page.reload();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(
    'Keep this unfinished brief for tomorrow',
  );
  await expect(page.locator('.creation-stepper [aria-current="step"]')).toContainText('Brief');
  expect(requests).toBe(0);
  const saved = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
  expect(saved.state.step).toBe('brief');
  expect(saved.journey.draft.nodes).toHaveLength(0);
});
test('review resumes and accepts a correction before approval opens the canvas', async ({
  page,
}) => {
  const id = await startGuided(page, 'Guided trial journey');
  await page.getByLabel('Your brief', { exact: true }).fill('Start on a cart event and finish.');
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveCount(0);
  await expect(page.getByLabel('What should happen?', { exact: true })).toHaveCount(0);
  await expect(page.locator('.vue-flow__node')).toHaveCount(0);
  await page.getByRole('button', { name: '← Journeys', exact: true }).click();
  await page.getByRole('button', { name: 'Guided trial journey', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  const revised = structuredClone(fixture);
  revised.workflow.nodes[1]!.label = 'Reviewed completion';
  await page.route('**/api/v1/ai/journeys/modify/stream', (route) => {
    const input = route.request().postDataJSON();
    expect(input.workflow).toEqual(fixture.workflow);
    expect(input.prompt).toBe('Rename the exit');
    return route.fulfill({
      contentType: 'text/event-stream',
      body: `data: ${JSON.stringify({ type: 'ready', proposal: revised })}\n\n`,
    });
  });
  await page.getByRole('button', { name: 'Request changes', exact: true }).click();
  await page.getByLabel('What should change?', { exact: true }).fill('Rename the exit');
  await page.getByRole('button', { name: 'Update plan', exact: true }).click();
  await expect(page.getByLabel('Journey plan')).toContainText('Reviewed completion');
  await expect
    .poll(async () => {
      const r = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
      return r.state.proposal?.workflow.nodes[1].label;
    })
    .toBe('Reviewed completion');
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route('**/api/v1/journeys/*/composition/approve', async (route) => {
    await gate;
    await route.continue();
  });
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(
    page.getByRole('progressbar', { name: 'Builder preparation progress' }),
  ).toBeVisible();
  release();
  await expect(page.locator('.canvas .vue-flow__node')).toHaveCount(2);
  const saved = await (await page.request.get(`/api/v1/journeys/${id}`)).json();
  expect(saved.creation_step).toBe('builder');
  expect(saved.status).toBe('Draft');
  expect(saved.draft).toEqual(revised.workflow);
});
test('failed approval retains review and retries without generation', async ({ page }) => {
  await startGuided(page, 'Approval retry');
  await page.getByLabel('Your brief', { exact: true }).fill('Create a short journey');
  let generations = 0;
  page.on('request', (request) => {
    if (request.url().includes('/ai/journeys/')) generations++;
  });
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  let attempts = 0;
  await page.route('**/api/v1/journeys/*/composition/approve', (route) => {
    attempts++;
    return attempts === 1
      ? route.fulfill({ status: 500, json: { detail: 'Temporary approval failure' } })
      : route.continue();
  });
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.locator('.creation-page [role="alert"]')).toContainText(
    'Temporary approval failure',
  );
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.locator('.canvas .vue-flow__node')).toHaveCount(2);
  expect(generations).toBe(1);
  expect(attempts).toBe(2);
});

test('interrupted creation restores its brief without restarting a paid request', async ({
  page,
}) => {
  await startGuided(page, 'Interrupted brief');
  let calls = 0,
    release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route('**/api/v1/ai/journeys/generate/stream', async (route) => {
    calls++;
    await gate;
    await route
      .fulfill({
        contentType: 'text/event-stream',
        body: `data: ${JSON.stringify({ type: 'ready', proposal: fixture })}\n\n`,
      })
      .catch(() => {});
  });
  await page
    .getByLabel('Your brief', { exact: true })
    .fill('Preserve this unique unfinished request');
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect.poll(() => calls).toBe(1);
  await expect(
    page.getByText('Preserve this unique unfinished request', { exact: true }),
  ).toHaveCount(1);
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveCount(0);
  await expect(page.getByLabel('Journey generation progress')).toBeVisible();
  page.once('dialog', (dialog) => dialog.accept());
  await page.reload();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(
    'Preserve this unique unfinished request',
  );
  await expect(page.locator('.creation-page')).toContainText('No AI request was restarted');
  release();
  expect(calls).toBe(1);
});

test('a failed composition restore cannot overwrite the saved brief', async ({ page }) => {
  const id = await startGuided(page, 'Restore safely');
  await page
    .getByLabel('Your brief', { exact: true })
    .fill('This saved text must survive a read outage');
  await page.getByRole('button', { name: '← Journeys', exact: true }).click();
  await page.getByRole('button', { name: 'Restore safely', exact: true }).click();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(
    'This saved text must survive a read outage',
  );
  let writes = 0;
  page.on('request', (request) => {
    if (request.url().endsWith('/composition') && request.method() === 'PUT') writes++;
  });
  await page.route(`**/api/v1/journeys/${id}/composition`, (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ status: 503, json: { detail: 'Composition unavailable' } })
      : route.continue(),
  );
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Creation draft unavailable', exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveCount(0);
  expect(writes).toBe(0);
  await page.unroute(`**/api/v1/journeys/${id}/composition`);
  await page.getByRole('button', { name: 'Retry loading', exact: true }).click();
  await expect(page.getByLabel('Your brief', { exact: true })).toHaveValue(
    'This saved text must survive a read outage',
  );
});
