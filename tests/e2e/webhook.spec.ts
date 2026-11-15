import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('a webhook step is configured on the canvas and recorded once per customer', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Integrator',
        workspace: 'Webhooks',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'CRM sync' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Signed up',
        position: { x: 0, y: 0 },
        config: { event_name: 'signed_up' },
      },
      {
        id: 'notify',
        type: 'webhook',
        label: 'Tell the CRM',
        position: { x: 0, y: 190 },
        config: {
          url: 'https://crm.example.invalid/hooks/journey',
          fields: { email: 'customer.email', plan: 'event.properties.plan' },
        },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 380 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'e1', source: 'start', target: 'notify', port: 'started' },
      { id: 'e2', source: 'notify', target: 'done', port: 'next' },
    ],
  };
  const saved = await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
    headers,
    data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
  });
  expect(saved.ok(), await saved.text()).toBeTruthy();
  const journey = await saved.json();

  await page.goto(`/journeys/${journey.id}`);
  const card = page.locator('.canvas .vue-flow__node[data-id="notify"]');
  await expect(card).toContainText('Webhook');
  await expect(card.locator('.workflow-node-detail')).toHaveText(
    'POST to crm.example.invalid · 2 fields',
  );
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await card.click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await expect(settings).toContainText('Only hosts the server allows are called');
  await expect(settings.getByLabel('Value of field 2', { exact: true })).toHaveValue(
    /^(customer.email|event.properties.plan)$/,
  );
  await settings.getByRole('button', { name: 'Add a field', exact: true }).click();
  await settings.getByLabel('Name of field 3', { exact: true }).fill('tier');
  await settings.getByLabel('Name of field 3', { exact: true }).blur();
  await settings.getByLabel('Value of field 3', { exact: true }).fill('customer.attributes.tier');
  await settings.getByLabel('Value of field 3', { exact: true }).blur();
  await expect(card.locator('.workflow-node-detail')).toHaveText(
    'POST to crm.example.invalid · 3 fields',
  );

  // An address the runtime could never call is caught before publishing.
  await settings.getByLabel('Address', { exact: true }).fill('ftp://user:secret@crm.example');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(
    settings.getByText('Enter an http or https address without credentials'),
  ).toBeVisible();
  await settings
    .getByLabel('Address', { exact: true })
    .fill('https://crm.example.invalid/hooks/journey');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const current = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(current.draft.nodes.find((node: { id: string }) => node.id === 'notify').config).toEqual({
    url: 'https://crm.example.invalid/hooks/journey',
    fields: {
      email: 'customer.email',
      plan: 'event.properties.plan',
      tier: 'customer.attributes.tier',
    },
  });

  // The sandbox only says what it would call.
  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        scenario: { name: 'Webhook gate', entry_event: { event_name: 'signed_up' } },
      },
    })
  ).json();
  const finished = await (
    await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
      headers,
      data: { operation: 'complete', revision: run.revision },
    })
  ).json();
  expect(JSON.stringify(finished)).toContain('Would call https://crm.example.invalid');

  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: current.revision },
  });
  expect(published.ok(), await published.text()).toBeTruthy();
  const customer = 'crm_' + randomBytes(4).toString('hex');
  await page.request.post('/api/v1/customers', {
    headers,
    data: { customer_id: customer, email: customer + '@example.invalid' },
  });
  const sent = await page.request.post('/api/v1/events', {
    headers,
    data: {
      event_id: randomBytes(12).toString('hex'),
      customer_id: customer,
      event_name: 'signed_up',
      occurred_at: new Date().toISOString(),
      properties: { plan: 'pro' },
    },
  });
  expect(sent.ok(), await sent.text()).toBeTruthy();

  // The demo server allows no hosts, so the call is kept as a record and never leaves.
  let detail: { webhooks: { node_id: string; state: string; url: string }[] } = { webhooks: [] };
  await expect(async () => {
    const executions = await (
      await page.request.get(
        `/api/v1/journeys/${journey.id}/executions?version=1&customer_id=${customer}`,
      )
    ).json();
    expect(executions).toHaveLength(1);
    detail = await (await page.request.get(`/api/v1/executions/${executions[0].id}`)).json();
    expect(detail.webhooks.map((call) => call.state)).toEqual(['RECORDED']);
  }).toPass({ timeout: 20_000 });
  expect(detail.webhooks[0]).toMatchObject({
    node_id: 'notify',
    url: 'https://crm.example.invalid/hooks/journey',
  });

  await page.goto(`/customers?customer=${customer}`);
  const route = page.getByRole('region', { name: 'Customer journey' }).locator('.route-step');
  await expect(route).toHaveCount(3);
  await expect(route.nth(1)).toContainText(
    'Webhook recorded; crm.example.invalid is not on the allowed list',
  );
  await expect(route.nth(1)).toContainText('POST to crm.example.invalid · 3 fields');
});
