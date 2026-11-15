import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('reviewed business checks survive AI layout and block a wrong channel after manual editing', async ({
  page,
}, testInfo) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Business checks',
        workspace: 'Business checks',
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
          aliases: [],
          enabled: true,
        },
        {
          kind: 'customer_field',
          path: 'customer.attributes.budget',
          label: 'Budget',
          data_type: 'number',
          aliases: [],
          enabled: true,
        },
      ],
    },
  });
  await page.route('**/api/v1/settings/ai', (route) =>
    route.fulfill({
      json: {
        enabled: true,
        default_provider: 'openai',
        providers: {
          openai: { configured: true, model: 'offline-fixture' },
          claude: { configured: false, model: '' },
        },
      },
    }),
  );
  let requests = 0;
  await page.route('**/api/v1/ai/journeys/generate/stream', (route) => {
    requests++;
    const request = route.request().postDataJSON();
    expect(request.behavior_checks).toMatchObject({
      confirmed: true,
      email_count: 1,
      sms_count: 0,
      entry_event: 'registered',
    });
    expect(request.behavior_checks.thresholds[0]).toMatchObject({
      operator: 'greater_than_or_equal',
      value: 10000,
    });
    const workflow = {
      schema_version: 1,
      behavior_checks: request.behavior_checks,
      nodes: [
        {
          id: 'start',
          type: 'event_trigger',
          label: 'Registered',
          position: { x: 0, y: 0 },
          config: { event_name: 'registered' },
        },
        {
          id: 'budget',
          type: 'condition',
          label: 'Budget qualifies?',
          position: { x: 0, y: 0 },
          config: {
            rule: {
              path: 'customer.attributes.budget',
              operator: 'greater_than_or_equal',
              value: 10000,
            },
          },
        },
        {
          id: 'email',
          type: 'email',
          label: 'Confirmation email',
          position: { x: 0, y: 0 },
          config: { subject: 'Confirmation', text: 'Welcome' },
        },
        {
          id: 'done',
          type: 'exit',
          label: 'Complete',
          position: { x: 0, y: 0 },
          config: { reason: 'completed' },
        },
        {
          id: 'ineligible',
          type: 'exit',
          label: 'Not eligible',
          position: { x: 0, y: 0 },
          config: { reason: 'condition_not_met' },
        },
      ],
      edges: [
        { id: 'a', source: 'start', target: 'budget', port: 'started' },
        { id: 'b', source: 'budget', target: 'email', port: 'true' },
        { id: 'c', source: 'budget', target: 'ineligible', port: 'false' },
        { id: 'd', source: 'email', target: 'done', port: 'next' },
      ],
    };
    return route.fulfill({
      contentType: 'text/event-stream',
      body: `data: ${JSON.stringify({ type: 'ready', proposal: { schema_version: 'composer.v5', action: 'generate', name: 'Checked journey', description: 'Independent expectations', workflow, explanation: 'One email for eligible budgets.', warnings: [], not_done: ['The brief gives the link https://example.com/start, but no message uses it.'], diff: { metadata_changed: true, edges_changed: true, exit_changed: false, reentry_changed: false, added_nodes: ['start', 'budget', 'email', 'done', 'ineligible'], removed_nodes: [], updated_nodes: [], changed_conditions: [], changed_messages: [] } } })}\n\n`,
    });
  });
  await page.goto('/journeys/new');
  await page.getByLabel('Journey name', { exact: true }).fill('Checked journey');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await page.getByRole('button', { name: 'Create with AI', exact: false }).click();
  await page
    .getByLabel('Your brief', { exact: true })
    .fill(
      'Send one email when budget is at least 10000; otherwise do not send. Complete normally.',
    );
  const generate = page.getByRole('button', { name: 'Generate plan', exact: false });
  await expect(generate).toBeDisabled();
  const checks = page.getByRole('region', { name: 'Independent behavior checks', exact: true });
  await checks.getByRole('combobox', { name: 'Expected entry', exact: true }).click();
  await page.getByRole('option', { name: 'Customer registered', exact: true }).click();
  await checks.getByLabel('Expected emails', { exact: true }).fill('1');
  await checks.getByLabel('Expected SMS', { exact: true }).fill('0');
  await checks.getByRole('button', { name: 'Add boundary', exact: true }).click();
  await checks.getByRole('combobox', { name: 'Boundary field 1', exact: true }).click();
  await page.getByRole('option', { name: 'Budget', exact: true }).click();
  await checks.getByLabel('Boundary value', { exact: true }).fill('10000');
  await checks
    .getByLabel('I reviewed these expectations and the synthetic example.', { exact: true })
    .check();
  await expect(generate).toBeEnabled();
  await generate.click();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('region', { name: 'Reviewed behavior expectations' })).toContainText(
    '1 email · 0 SMS',
  );
  await expect(
    page.getByRole('region', { name: 'In the brief, not in the draft', exact: true }),
  ).toContainText('https://example.com/start, but no message uses it');
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.getByText('Editing draft', { exact: true })).toBeVisible();
  const id = new URL(page.url()).pathname.split('/').at(-1)!;
  const saved = await (await page.request.get('/api/v1/journeys/' + id)).json();
  expect(saved.draft.behavior_checks).toMatchObject({
    confirmed: true,
    email_count: 1,
    sms_count: 0,
  });
  const positions = saved.draft.nodes.map(
    (n: { position: { x: number; y: number } }) => n.position,
  );
  for (let i = 0; i < positions.length; i++)
    for (let j = i + 1; j < positions.length; j++)
      expect(
        Math.abs(positions[i].x - positions[j].x) >= 248 ||
          Math.abs(positions[i].y - positions[j].y) >= 220,
      ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath('checked-generated-canvas.png'),
    fullPage: true,
  });
  const w = structuredClone(saved.draft);
  w.nodes.find((n: { id: string }) => n.id === 'email').type = 'sms';
  const response = await page.request.put('/api/v1/journeys/' + id + '/draft', {
    headers,
    data: { name: saved.name, description: saved.description, revision: saved.revision, draft: w },
  });
  expect(response.ok()).toBeTruthy();
  const validation = await (
    await page.request.post('/api/v1/journeys/' + id + '/validate', { headers, data: {} })
  ).json();
  expect(validation.valid).toBe(false);
  expect(
    validation.issues.some((i: { field: string }) => i.field === 'behavior_checks.email_count'),
  ).toBeTruthy();
  const gate = await (await page.request.get('/api/v1/journeys/' + id + '/publish-check')).json();
  expect(gate.ready).toBe(false);
  expect(requests).toBe(1);
});
