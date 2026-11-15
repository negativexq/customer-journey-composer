import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('a multi-way branch sends customers down the first matching path', async ({ page }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Segmenter',
        workspace: 'Multi-way',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Tier offers' } })
  ).json();
  const tier = (value: string) => ({
    path: 'customer.attributes.tier',
    operator: 'equals',
    value,
  });
  const exit = (id: string, label: string, x: number) => ({
    id,
    type: 'exit',
    label,
    position: { x, y: 420 },
    config: { reason: 'completed' },
  });
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart updated',
        position: { x: 300, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'tiers',
        type: 'switch',
        label: 'Which tier?',
        position: { x: 300, y: 190 },
        config: {
          branches: [
            { label: 'Gold', rule: tier('gold') },
            { label: 'Silver', rule: tier('silver') },
          ],
        },
      },
      exit('gold', 'Gold offer', 0),
      exit('silver', 'Silver offer', 300),
      exit('other', 'Standard offer', 600),
    ],
    edges: [
      { id: 'e1', source: 'start', target: 'tiers', port: 'started' },
      { id: 'e2', source: 'tiers', target: 'gold', port: 'case_1' },
      { id: 'e3', source: 'tiers', target: 'silver', port: 'case_2' },
      { id: 'e4', source: 'tiers', target: 'other', port: 'else' },
    ],
  };
  const saved = await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
    headers,
    data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
  });
  expect(saved.ok(), await saved.text()).toBeTruthy();
  const journey = await saved.json();
  const current = async () =>
    (await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()).draft as typeof draft;

  // The sandbox takes the first branch that matches, and Otherwise when none does.
  const land = async (value: string | undefined) => {
    const run = await (
      await page.request.post('/api/v1/test-runs', {
        headers,
        data: {
          journey_id: journey.id,
          scenario: {
            name: 'Tier ' + value,
            customer: { customer_id: 'shopper', attributes: value ? { tier: value } : {} },
            entry_event: { event_name: 'cart_updated' },
          },
        },
      })
    ).json();
    const finished = await (
      await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
        headers,
        data: { operation: 'complete', revision: run.revision },
      })
    ).json();
    return finished.state.current_node_id;
  };
  expect(await land('gold')).toBe('gold');
  expect(await land('silver')).toBe('silver');
  expect(await land('bronze')).toBe('other');
  expect(await land(undefined)).toBe('other');

  await page.goto(`/journeys/${journey.id}`);
  const card = page.locator('.canvas .vue-flow__node[data-id="tiers"]');
  await expect(card).toContainText('Multi-way Branch');
  await expect(card.locator('.workflow-node-detail')).toHaveText('Gold · Silver · otherwise');
  await expect(card.locator('.vue-flow__handle.source')).toHaveCount(3);
  const caption = (id: string) => page.locator(`.canvas [data-id="${id}"] .node-caption`);
  await expect(caption('gold')).toHaveText('If Gold');
  await expect(caption('silver')).toHaveText('If Silver');
  await expect(caption('other')).toHaveText('Otherwise');

  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await card.click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await expect(settings).toContainText('the first one that matches');
  await settings.getByLabel('Name of branch 2', { exact: true }).fill('Silver members');
  await expect(caption('silver')).toHaveText('If Silver members');

  // A new branch gets its own output, between the existing branches and Otherwise.
  await settings.getByRole('button', { name: 'Add a branch', exact: true }).click();
  await expect(card.locator('.vue-flow__handle.source')).toHaveCount(4);
  await expect(card.locator('.workflow-node-detail')).toHaveText(
    'Gold · Silver members · Branch 3 · otherwise',
  );
  await expect(
    page.getByRole('button', { name: 'Add a step after Which tier? · Branch 3', exact: true }),
  ).toBeVisible();

  // Removing the first branch keeps the others connected to the same steps.
  await settings.getByRole('button', { name: 'Remove branch 1', exact: true }).click();
  await expect(card.locator('.workflow-node-detail')).toHaveText(
    'Silver members · Branch 3 · otherwise',
  );
  await expect(caption('silver')).toHaveText('If Silver members');
  await expect(caption('other')).toHaveText('Otherwise');
  await expect(caption('gold')).toHaveCount(0);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();
  const edited = await current();
  expect(
    edited.edges.filter((edge) => edge.source === 'tiers').map((edge) => [edge.port, edge.target]),
  ).toEqual([
    ['case_1', 'silver'],
    ['else', 'other'],
  ]);

  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(card.locator('.workflow-node-detail')).toHaveText(
    'Gold · Silver members · otherwise',
  );
  await expect(caption('gold')).toHaveText('If Gold');
});
