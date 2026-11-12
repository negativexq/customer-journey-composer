import { confirmEmptyBehavior } from './helpers/behavior';
import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('business brief mappings require workspace definitions and resume after explicit choices', async ({
  page,
}, testInfo) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Data reviewer',
        workspace: 'Vocabulary tests',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  await page.request.put('/api/v1/data-catalog', {
    headers,
    data: {
      entries: [
        {
          kind: 'event',
          event_name: 'registered',
          label: 'Customer registered',
          data_type: 'event',
          aliases: ['Registration'],
          enabled: true,
        },
        {
          kind: 'customer_field',
          path: 'customer.attributes.active',
          label: 'Account active',
          data_type: 'boolean',
          aliases: ['Activation'],
          enabled: true,
        },
        {
          kind: 'customer_field',
          path: 'customer.attributes.enabled',
          label: 'Account active',
          data_type: 'boolean',
          aliases: ['Activation'],
          enabled: true,
        },
      ],
    },
  });
  const workflow = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Customer registers',
        position: { x: 0, y: 0 },
        config: { event_name: 'user_registered' },
      },
      {
        id: 'check',
        type: 'condition',
        label: 'Account active?',
        position: { x: 0, y: 200 },
        config: {
          rule: { path: 'customer.attributes.activated', operator: 'equals', value: true },
        },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Complete',
        position: { x: 0, y: 400 },
        config: { reason: 'completed' },
      },
      {
        id: 'converted',
        type: 'exit',
        label: 'Active',
        position: { x: 350, y: 400 },
        config: { reason: 'converted' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'check', port: 'started' },
      { id: 'b', source: 'check', target: 'done', port: 'false' },
      { id: 'c', source: 'check', target: 'converted', port: 'true' },
    ],
  };
  await page.route('**/api/v1/settings/ai', (route) =>
    route.fulfill({
      json: {
        enabled: true,
        default_provider: 'openai',
        providers: {
          openai: { configured: true, model: 'fixture' },
          claude: { configured: true, model: 'fixture' },
        },
      },
    }),
  );
  let paid = 0;
  await page.route('**/api/v1/ai/**', (route) => {
    paid++;
    return route.fulfill({
      contentType: 'text/event-stream',
      body: `data: ${JSON.stringify({ type: 'ready', proposal: { schema_version: 'composer.v2', action: 'generate', diff: { metadata_changed: true, added_nodes: ['start', 'check', 'done', 'converted'], removed_nodes: [], updated_nodes: [], changed_conditions: [], changed_messages: [], edges_changed: true, exit_changed: false, reentry_changed: false }, name: 'Account activation', description: 'Business-language onboarding', workflow, explanation: 'Registration checks account activation.', warnings: [] } })}\n\n`,
    });
  });
  await page.goto('/journeys/new');
  await page.getByLabel('Journey name', { exact: true }).fill('Business onboarding');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await page.getByRole('button', { name: 'Create with AI', exact: false }).click();
  await page
    .getByLabel('Your brief', { exact: true })
    .fill('When a customer registers, check whether their account is active.');
  await confirmEmptyBehavior(page);
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  const mappings = page.getByRole('region', { name: 'Data used by this plan' });
  await expect(mappings).toContainText('Not connected');
  const approve = page.getByRole('button', { name: 'Approve & open builder', exact: true });
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await expect(approve).toBeDisabled();
  const id = page.url().match(/journeys\/([^/?]+)/)![1];
  const composition = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
  const rejected = await page.request.post(`/api/v1/journeys/${id}/composition/approve`, {
    headers,
    data: { revision: composition.journey.revision },
  });
  expect(rejected.status()).toBe(422);
  await mappings
    .getByLabel('Definition for event:user_registered:', { exact: true })
    .selectOption('event:registered:');
  await mappings
    .getByLabel('Definition for customer_field::customer.attributes.activated', { exact: true })
    .selectOption('customer_field::customer.attributes.active');
  await mappings.getByRole('button', { name: 'Use selected definitions', exact: true }).click();
  await expect(mappings).toContainText('Your choice');
  await expect(mappings).toContainText('All references have an available definition');
  await expect(page.getByRole('status').filter({ hasText: 'All changes saved' })).toBeVisible();
  await page.reload();
  await expect(mappings).toContainText('Your choice');
  await page.screenshot({ path: testInfo.outputPath('data-mapping-review.png'), fullPage: true });
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await approve.click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(4);
  const saved = await (await page.request.get(`/api/v1/journeys/${id}`)).json();
  expect(saved.draft.nodes.find((n: { id: string }) => n.id === 'start').config.event_name).toBe(
    'registered',
  );
  expect(saved.draft.nodes.find((n: { id: string }) => n.id === 'check').config.rule.path).toBe(
    'customer.attributes.active',
  );
  expect(saved.status).toBe('Draft');
  expect(paid).toBe(1); // One intercepted request; no external provider call.
});

test('workspace data definitions are editable in Settings without changing customer values', async ({
  page,
}, testInfo) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Catalog editor',
        workspace: 'Catalog UI',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  await page.request.post('/api/v1/customers', {
    headers: { 'X-CSRF-Token': user.csrf_token },
    data: { customer_id: 'catalog-demo', attributes: { activated: true } },
  });
  await page.goto('/settings');
  const catalog = page.getByRole('region', { name: 'Workspace data catalog' });
  await catalog.getByRole('button', { name: 'Discover recent data', exact: true }).click();
  await expect(catalog).toContainText('customer.attributes.activated');
  await catalog.getByLabel('Find a definition', { exact: true }).fill('activated');
  await catalog.getByRole('button', { name: 'Edit', exact: true }).click();
  await catalog.getByLabel('Business name', { exact: true }).fill('Account active');
  await catalog.getByLabel('Also called', { exact: true }).fill('Activation');
  await catalog.getByRole('button', { name: 'Save definition', exact: true }).click();
  await catalog.getByLabel('Find a definition', { exact: true }).fill('Account active');
  await expect(catalog.getByRole('table')).toContainText('Account active');
  await page.reload();
  await expect(catalog.getByRole('table')).toContainText('Account active');
  await page.screenshot({ path: testInfo.outputPath('workspace-data.png'), fullPage: true });
  const customer = await (await page.request.get('/api/v1/customers/catalog-demo')).json();
  expect(customer.attributes.activated).toBe(true);
});
