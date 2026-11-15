import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('readable node summaries and connection focus preserve graph semantics and layout undo', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Canvas reviewer',
        workspace: 'Readable canvas',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', {
      headers,
      data: { name: 'Readable branch review' },
    })
  ).json();
  const workflow = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Trial started',
        position: { x: 100, y: 0 },
        config: { event_name: 'trial_started' },
      },
      {
        id: 'check',
        type: 'condition',
        label: 'Needs a guide?',
        position: { x: 100, y: 200 },
        config: {
          rule: {
            logic: 'and',
            rules: [
              { path: 'customer.attributes.login_count', operator: 'less_than', value: 3 },
              { path: 'customer.consents.email', operator: 'equals', value: true },
            ],
          },
        },
      },
      {
        id: 'wait',
        type: 'wait_event',
        label: 'Wait for subscription',
        position: { x: 0, y: 440 },
        config: { event_name: 'subscription_started', timeout_seconds: 172800 },
      },
      {
        id: 'converted',
        type: 'exit',
        label: 'Subscribed',
        position: { x: 0, y: 720 },
        config: { reason: 'converted' },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'No action',
        position: { x: 450, y: 1100 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'begin', source: 'start', target: 'check', port: 'started' },
      { id: 'yes', source: 'check', target: 'wait', port: 'true' },
      { id: 'no', source: 'check', target: 'done', port: 'false' },
      { id: 'matched', source: 'wait', target: 'converted', port: 'matched' },
      { id: 'timeout', source: 'wait', target: 'done', port: 'timeout' },
    ],
  };
  await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
    headers,
    data: { name: journey.name, description: '', draft: workflow, revision: journey.revision },
  });
  await page.goto(`/journeys/${journey.id}`);
  await expect(page.locator('.canvas-zoom-level')).toContainText('90%');
  await expect(page.locator('[data-id="check"] .workflow-node-detail')).toContainText(
    'login_count < 3 AND consent.email = true',
  );
  await expect(page.locator('[data-id="wait"] .workflow-node-detail')).toContainText(
    'up to 2 days',
  );
  await page.locator('.canvas .vue-flow__node[data-id="check"]').click();
  const inspector = page.getByLabel('Step settings', { exact: true });
  const more = page.getByLabel('More canvas actions', { exact: true });
  await more.click();
  await page.getByLabel('Short exit links', { exact: true }).check();
  await more.click();
  // On a phone the settings panel opens over the canvas; hide it to reach the canvas again.
  if (test.info().project.name === 'mobile')
    await page.getByRole('button', { name: 'Toggle step settings', exact: true }).click();
  await page
    .getByRole('button', {
      name: 'Go to shared exit · Needs a guide? → No action · No',
      exact: true,
    })
    .click();

  await expect(inspector.getByLabel('Step name', { exact: true })).toHaveValue('No action');
  await expect(page.locator('.canvas .workflow-node.is-muted')).not.toHaveCount(0);
  await more.click();
  await page.getByRole('button', { name: 'Go to start', exact: true }).click();
  await expect(page.locator('.canvas .workflow-node.is-muted')).toHaveCount(0);
  const before = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(before.draft).toEqual(workflow);
  await more.click();
  await page.getByRole('button', { name: 'Auto layout', exact: true }).click();
  await expect(page.locator('.canvas-zoom-level')).toContainText('90%');
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const restored = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(restored.draft).toEqual(workflow);
});
