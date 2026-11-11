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
        requested_model: 'contract-fixture',
        model: 'contract-fixture',
        duration_ms: 1,
        usage: { input_tokens: 12, output_tokens: 4, reasoning_tokens: 0 },
      },
    ],
    usage: { input_tokens: 12, output_tokens: 4, reasoning_tokens: 0 },
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
  await page.route('**/api/v1/ai/journeys/generate', (route) => route.fulfill({ json: fixture }));
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
