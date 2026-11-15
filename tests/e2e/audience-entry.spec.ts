import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('a journey can enter a scheduled audience instead of waiting for an event', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Segmenter',
        workspace: 'Audiences',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Gold weekly' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Gold members',
        position: { x: 0, y: 0 },
        config: {
          entry_mode: 'audience',
          event_name: 'audience_entry',
          filters: { path: 'customer.attributes.tier', operator: 'equals', value: 'gold' },
          at_time: '09:00',
          timezone: 'UTC',
        },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 200 },
        config: { reason: 'completed' },
      },
    ],
    edges: [{ id: 'e1', source: 'start', target: 'done', port: 'started' }],
  };
  const saved = await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
    headers,
    data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
  });
  expect(saved.ok(), await saved.text()).toBeTruthy();
  const journey = await saved.json();

  await page.goto(`/journeys/${journey.id}`);
  const card = page.locator('.canvas .vue-flow__node[data-id="start"]');
  await expect(card.locator('.workflow-node-detail')).toHaveText('Audience at 09:00 every day');
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await card.click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await expect(settings.getByLabel('Entry type', { exact: true })).toHaveValue('audience');
  await expect(settings.getByLabel('Event name', { exact: true })).toHaveCount(0);
  await expect(settings.getByText('Who enters', { exact: true })).toBeVisible();
  await settings.getByRole('button', { name: 'Mon', exact: true }).click();
  await expect(card.locator('.workflow-node-detail')).toHaveText('Audience at 09:00 on Mon');

  // Switching back to an event entry clears the audience; undo restores it.
  await settings.getByLabel('Entry type', { exact: true }).selectOption('event');
  await expect(settings.getByLabel('Event name', { exact: true })).toHaveValue('cart_updated');
  await expect(card.locator('.workflow-node-detail')).toHaveText('cart_updated');
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(card.locator('.workflow-node-detail')).toHaveText('Audience at 09:00 on Mon');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('All changes saved', { exact: true })).toBeVisible();

  // The sandbox treats the audience rule as the entry check.
  const sandbox = async (tier: string) => {
    const run = await (
      await page.request.post('/api/v1/test-runs', {
        headers,
        data: {
          journey_id: journey.id,
          scenario: {
            name: 'Audience ' + tier,
            customer: { customer_id: 'member', attributes: { tier } },
            entry_event: { event_name: 'audience_entry' },
          },
        },
      })
    ).json();
    return (
      await (
        await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
          headers,
          data: { operation: 'complete', revision: run.revision },
        })
      ).json()
    ).state.status;
  };
  expect(await sandbox('gold')).not.toBe('NOT_ENROLLED');

  const current = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: current.revision },
  });
  expect(published.ok(), await published.text()).toBeTruthy();
  // Run after publishing: an ineligible sandbox case would block the publish gate.
  expect(await sandbox('silver')).toBe('NOT_ENROLLED');
  const suffix = randomBytes(3).toString('hex');
  for (const [id, tier] of [
    ['gold_a_' + suffix, 'gold'],
    ['gold_b_' + suffix, 'gold'],
    ['silver_' + suffix, 'silver'],
  ])
    await page.request.post('/api/v1/customers', {
      headers,
      data: { customer_id: id, attributes: { tier } },
    });

  await page.goto(`/journeys/${journey.id}?tab=live-monitor`);
  const panel = page.getByRole('region', { name: 'Audience entry' });
  await expect(panel).toContainText('Next run');
  await expect(panel).toContainText('No runs yet.');
  await panel.getByRole('button', { name: 'Enter matching customers now', exact: true }).click();
  await expect(
    page.getByText('2 customers entered · 2 matched the audience').first(),
  ).toBeVisible();
  await expect(panel.locator('.audience-runs li')).toHaveCount(1);
  await expect(panel.locator('.audience-runs li')).toContainText('Started by hand');
  await expect(panel.locator('.audience-runs li')).toContainText(
    '2 customers entered · 2 matched of 3 checked',
  );

  await page.goto(`/customers?customer=gold_a_${suffix}`);
  const route = page.getByRole('region', { name: 'Customer journey' }).locator('.route-step');
  await expect(route.first()).toContainText('Entered with the scheduled audience');
});
