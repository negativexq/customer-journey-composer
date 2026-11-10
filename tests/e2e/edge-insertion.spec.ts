import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('edge insertion preserves the original branch, requires an explicit new branch and undoes atomically', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Canvas editor',
        workspace: 'Edge insertion',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Branch insertion' } })
  ).json();
  const node = (id: string, type: string, x: number, y: number, config: unknown) => ({
    id,
    type,
    label: id,
    position: { x, y },
    config,
  });
  const workflow = {
    schema_version: 1,
    nodes: [
      node('start', 'event_trigger', 100, 0, { event_name: 'cart_updated' }),
      node('check', 'condition', 100, 160, {
        rule: { path: 'customer.attributes.purchased', operator: 'equals', value: true },
      }),
      node('yes', 'exit', 0, 350, { reason: 'converted' }),
      node('no', 'exit', 300, 350, { reason: 'completed' }),
    ],
    edges: [
      { id: 'start_check', source: 'start', target: 'check', port: 'started' },
      { id: 'yes_branch', source: 'check', target: 'yes', port: 'true' },
      { id: 'no_branch', source: 'check', target: 'no', port: 'false' },
    ],
  };
  await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
    headers,
    data: {
      name: journey.name,
      description: '',
      tags: [],
      draft: workflow,
      revision: journey.revision,
    },
  });
  await page.goto(`/journeys/${journey.id}`);
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.getByRole('button', { name: 'Insert step · check → no · No', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Insert a step', exact: true });
  await expect(dialog.getByLabel('Step type')).toHaveValue('wait');
  await dialog.getByRole('button', { name: 'Insert step', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(5);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const saved = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  const incoming = saved.draft.edges.find((e: { id: string }) => e.id === 'no_branch');
  expect(incoming.port).toBe('false');
  expect(incoming.target).not.toBe('no');
  expect(saved.draft.edges).toContainEqual(
    expect.objectContaining({ source: incoming.target, target: 'no', port: 'completed' }),
  );
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(4);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const restored = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(restored.draft.edges).toEqual(workflow.edges);
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await page.getByRole('button', { name: 'Insert step · check → no · No', exact: true }).click();
  await dialog.getByLabel('Step type').selectOption('condition');
  await expect(dialog.getByRole('button', { name: 'Insert step', exact: true })).toBeDisabled();
  await dialog.getByLabel('Continue through').selectOption('false');
  await expect(
    dialog.getByText('Connect the other output before publishing.', { exact: false }),
  ).toBeVisible();
  await dialog.getByRole('button', { name: 'Insert step', exact: true }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const branched = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  const inserted = branched.draft.nodes.find(
    (n: { id: string }) => !workflow.nodes.some((old) => old.id === n.id),
  );
  expect(branched.draft.edges).toContainEqual(
    expect.objectContaining({ source: inserted.id, target: 'no', port: 'false' }),
  );
  expect(
    branched.draft.edges.filter((e: { source: string }) => e.source === inserted.id),
  ).toHaveLength(1);
  const validation = await (
    await page.request.post(`/api/v1/journeys/${journey.id}/validate`, { headers })
  ).json();
  expect(validation.valid).toBe(false);
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(page.locator('.vue-flow__node')).toHaveCount(4);
});
