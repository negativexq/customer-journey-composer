import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('scoped contract resumes alternatives and enforces matched-event obligations', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Scoped contract',
        workspace: 'Scoped contract',
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
          event_name: 'request_received',
          label: 'Request received',
          data_type: 'event',
          aliases: [],
          enabled: true,
        },
        {
          kind: 'event',
          event_name: 'resource_ready',
          label: 'Resource available',
          data_type: 'event',
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
    const input = route.request().postDataJSON();
    expect(input.behavior_checks).toMatchObject({
      version: 2,
      case_name: 'No subsequent event',
      email_count: 0,
      sms_count: 0,
      alternatives: [
        {
          name: 'Event arrives',
          email_count: 1,
          events: [{ event_name: 'resource_ready', after_seconds: 600 }],
        },
      ],
    });
    const workflow = {
      schema_version: 1,
      behavior_checks: input.behavior_checks,
      nodes: [
        {
          id: 'a',
          type: 'event_trigger',
          label: 'Request',
          position: { x: 0, y: 0 },
          config: { event_name: 'request_received' },
        },
        {
          id: 'b',
          type: 'wait_event',
          label: 'Wait for resource',
          position: { x: 0, y: 0 },
          config: { event_name: 'resource_ready', timeout_seconds: 3600 },
        },
        {
          id: 'c',
          type: 'email',
          label: 'Notification',
          position: { x: 0, y: 0 },
          config: { subject: 'Available', text: 'Ready' },
        },
        {
          id: 'd',
          type: 'exit',
          label: 'Complete',
          position: { x: 0, y: 0 },
          config: { reason: 'completed' },
        },
      ],
      edges: [
        { id: '1', source: 'a', target: 'b', port: 'started' },
        { id: '2', source: 'b', target: 'c', port: 'matched' },
        { id: '3', source: 'b', target: 'd', port: 'timeout' },
        { id: '4', source: 'c', target: 'd', port: 'next' },
      ],
    };
    const proposal = {
      schema_version: 'composer.v5',
      action: 'generate',
      name: 'Scoped journey',
      description: 'Distinct event and timeout obligations',
      workflow,
      explanation: 'Different obligations for different event sequences.',
      warnings: [],
      diff: {
        metadata_changed: true,
        edges_changed: true,
        exit_changed: false,
        reentry_changed: false,
        added_nodes: ['a', 'b', 'c', 'd'],
        removed_nodes: [],
        updated_nodes: [],
        changed_conditions: [],
        changed_messages: [],
      },
    };
    return route.fulfill({
      contentType: 'text/event-stream',
      body: `data: ${JSON.stringify({ type: 'ready', proposal })}\n\n`,
    });
  });
  await page.goto('/journeys/new');
  await page.getByLabel('Journey name', { exact: true }).fill('Scoped journey');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await page.getByRole('button', { name: 'Create with AI', exact: false }).click();
  await page
    .getByLabel('Your brief', { exact: true })
    .fill(
      'Wait one hour. Notify by email if the resource becomes available. Otherwise complete without a message.',
    );
  const root = page.getByRole('region', { name: 'Independent behavior checks', exact: true });
  await root.getByLabel('Situation name', { exact: true }).fill('No subsequent event');
  await root.getByRole('combobox', { name: 'Expected entry', exact: true }).click();
  await page.getByRole('option', { name: 'Request received', exact: true }).click();
  await root.getByLabel('Expected emails', { exact: true }).fill('0');
  await root.getByLabel('Expected SMS', { exact: true }).fill('0');
  await root.getByLabel('Expected duration (hours, optional)', { exact: true }).fill('1');
  await root.getByRole('button', { name: 'Add alternative situation', exact: true }).click();
  const alt = page.getByRole('region', { name: 'Alternative contract clause', exact: true });
  await alt.getByLabel('Situation name', { exact: true }).fill('Event arrives');
  await alt.getByLabel('Expected emails', { exact: true }).fill('1');
  await alt.getByLabel('Expected SMS', { exact: true }).fill('0');
  await alt.getByLabel('Expected duration (hours, optional)', { exact: true }).fill('0.166666667');
  await alt.getByRole('button', { name: 'Add subsequent event', exact: true }).click();
  await alt.getByRole('combobox', { name: 'Follow-up event 1', exact: true }).click();
  await page.getByRole('option', { name: 'Resource available', exact: true }).click();
  await alt.getByLabel('Hours after entry', { exact: true }).fill('0.166666667');
  await root
    .getByLabel('I reviewed these expectations and the synthetic example.', { exact: true })
    .check();
  const id = page.url().split('/').at(-1)!;
  await expect
    .poll(async () => {
      const saved = await (await page.request.get(`/api/v1/journeys/${id}/composition`)).json();
      return saved.state.behavior_checks?.alternatives?.[0]?.events?.[0]?.after_seconds;
    })
    .toBe(600);
  await page.reload();
  await expect(alt.getByLabel('Situation name', { exact: true })).toHaveValue('Event arrives');
  await expect(alt.getByLabel('Hours after entry', { exact: true })).toHaveValue(
    '0.16666666666666666',
  );
  await page.getByRole('button', { name: 'Generate plan', exact: false }).click();
  await expect(
    page.getByRole('heading', { name: 'Review your journey', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('region', { name: 'Reviewed behavior expectations' })).toContainText(
    'Event arrives',
  );
  await page.getByLabel('I reviewed the flow, messages and assumptions').check();
  await page.getByRole('button', { name: 'Approve & open builder', exact: true }).click();
  await expect(page.locator('.canvas .vue-flow__node')).toHaveCount(4);
  const saved = await (await page.request.get(`/api/v1/journeys/${id}`)).json();
  const draft = saved.draft;
  // Keep the node reachable but route only the matched-event branch past it.
  // A wrong channel still passes the no-event clause; the other clause must fail.
  draft.nodes.find((n: { id: string }) => n.id === 'c').type = 'sms';
  const update = await page.request.put(`/api/v1/journeys/${id}/draft`, {
    headers,
    data: { name: saved.name, description: saved.description, revision: saved.revision, draft },
  });
  expect(update.ok()).toBeTruthy();
  const validation = await (
    await page.request.post(`/api/v1/journeys/${id}/validate`, { headers, data: {} })
  ).json();
  expect(validation.valid).toBe(false);
  expect(
    validation.issues.some((i: { field: string }) =>
      i.field.startsWith('behavior_checks.alternatives.0.'),
    ),
  ).toBeTruthy();
  const gate = await (await page.request.get(`/api/v1/journeys/${id}/publish-check`)).json();
  expect(gate.ready).toBe(false);
  expect(requests).toBe(1);
});
