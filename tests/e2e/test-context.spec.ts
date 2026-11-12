import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
test('Test Lab prepares onboarding inputs and reviewed checks from the selected workflow', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Context reviewer',
        workspace: 'Context tests',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Onboarding context' } })
  ).json();
  const workflow = {
    schema_version: 1,
    reentry: { mode: 'once' },
    global_exit: { event_name: 'account_activated' },
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Registered',
        position: { x: 0, y: 0 },
        config: { event_name: 'user_registered' },
      },
      {
        id: 'check',
        type: 'condition',
        label: 'Already active?',
        position: { x: 0, y: 200 },
        config: {
          rule: { path: 'customer.attributes.activated', operator: 'equals', value: true },
        },
      },
      {
        id: 'email',
        type: 'email',
        label: 'Welcome',
        position: { x: 0, y: 400 },
        config: {
          subject: 'Welcome {{ customer.attributes.product_name }}',
          text: 'Hi {{ customer.attributes.first_name }}',
        },
      },
      {
        id: 'wait',
        type: 'wait_event',
        label: 'Wait for activation',
        position: { x: 0, y: 600 },
        config: { event_name: 'account_activated', timeout_seconds: 172800 },
      },
      {
        id: 'exit',
        type: 'exit',
        label: 'Activated',
        position: { x: 350, y: 800 },
        config: { reason: 'converted' },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Completed',
        position: { x: 0, y: 800 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'check', port: 'started' },
      { id: 'b', source: 'check', target: 'exit', port: 'true' },
      { id: 'c', source: 'check', target: 'email', port: 'false' },
      { id: 'd', source: 'email', target: 'wait', port: 'next' },
      { id: 'e', source: 'wait', target: 'exit', port: 'matched' },
      { id: 'f', source: 'wait', target: 'done', port: 'timeout' },
    ],
  };
  await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
    headers,
    data: { name: journey.name, description: '', draft: workflow, revision: journey.revision },
  });
  await page.goto(`/journeys/${journey.id}?tab=preview-test&view=test-lab`);
  const lab = page.locator('.test-lab');
  await expect(lab.getByLabel('Entry event name', { exact: true })).toHaveValue('user_registered');
  await expect(
    lab.getByLabel('Sample customer.attributes.activated', { exact: true }),
  ).not.toBeChecked();
  await expect(
    lab.getByLabel('Sample customer.attributes.product_name', { exact: true }),
  ).toHaveValue('Sample value');
  await expect(
    lab.getByLabel('Exploratory run (does not qualify for publishing)', { exact: false }),
  ).toBeChecked();
  await lab.getByLabel('Entry event name', { exact: true }).fill('cart_updated');
  await expect(lab.getByRole('button', { name: 'Create test run', exact: true })).toBeDisabled();
  await expect(lab).toContainText('This journey expects user_registered');
  await lab.getByLabel('Entry event name', { exact: true }).fill('user_registered');
  await lab.getByLabel('Suggested check', { exact: true }).selectOption('1');
  await expect(lab).toContainText('Sends the global exit event after one second');
  await lab.getByRole('button', { name: 'Save scenario', exact: true }).click();
  await expect(lab.getByRole('alert')).toContainText('Review the suggested inputs');
  await lab.getByLabel('I reviewed the sample inputs and expected checks').check();
  await lab.getByRole('button', { name: 'Save scenario', exact: true }).click();
  await expect(lab.getByRole('status')).toContainText('Scenario saved');
  await lab.getByRole('button', { name: 'Create test run', exact: true }).click();
  await lab.getByRole('button', { name: 'Run to completion', exact: true }).click();
  await expect(lab.getByTestId('test-result')).toHaveText('PASS');
  await expect(lab).toContainText('converted');
  expect(
    await (await page.request.get(`/api/v1/journeys/${journey.id}/executions`)).json(),
  ).toEqual([]);
});
