import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('an A/B split assigns customers to variants and compares them in analytics', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Experimenter',
        workspace: 'Experiments',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const created = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Offer test' } })
  ).json();
  const draft = {
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Cart updated',
        position: { x: 200, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'offer',
        type: 'split',
        label: 'Offer test',
        position: { x: 200, y: 190 },
        config: { percent: 50 },
      },
      {
        id: 'won',
        type: 'exit',
        label: 'Bought with offer',
        position: { x: 40, y: 400 },
        config: { reason: 'converted' },
      },
      {
        id: 'plain',
        type: 'exit',
        label: 'No offer',
        position: { x: 360, y: 400 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'e1', source: 'start', target: 'offer', port: 'started' },
      { id: 'e2', source: 'offer', target: 'won', port: 'a' },
      { id: 'e3', source: 'offer', target: 'plain', port: 'b' },
    ],
  };
  const saved = await page.request.put(`/api/v1/journeys/${created.id}/draft`, {
    headers,
    data: { name: created.name, description: '', tags: [], revision: created.revision, draft },
  });
  expect(saved.ok()).toBeTruthy();
  const journey = await saved.json();

  // The builder shows the split and names each variant on the step it leads to.
  await page.goto(`/journeys/${journey.id}`);
  const split = page.locator('.canvas .vue-flow__node[data-id="offer"]');
  await expect(split).toContainText('A/B Split');
  await expect(split.locator('.workflow-node-detail')).toHaveText('50% A · 50% B');
  await expect(page.locator('.canvas [data-id="won"] .node-caption')).toHaveText(
    'Variant A · 50% of customers',
  );
  await expect(page.locator('.canvas [data-id="plain"] .node-caption')).toHaveText(
    'Variant B · 50% of customers',
  );
  await page.getByRole('button', { name: 'Fit view', exact: true }).click();
  await split.click();
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  await expect(settings).toContainText('A customer always gets the same variant here');
  await settings.getByLabel('Share for variant A (%)', { exact: true }).fill('70');
  await expect(settings.locator('.split-share')).toHaveText('A 70%B 30%');
  await expect(split.locator('.workflow-node-detail')).toHaveText('70% A · 30% B');
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(split.locator('.workflow-node-detail')).toHaveText('50% A · 50% B');
  await page.getByRole('button', { name: 'Validate', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Workflow is valid');

  const run = await (
    await page.request.post('/api/v1/test-runs', {
      headers,
      data: {
        journey_id: journey.id,
        // The sandbox customer is assigned like anyone else: always the same variant, here A,
        // which leaves through the converting exit.
        scenario: {
          name: 'Split gate',
          entry_event: { event_name: 'cart_updated' },
          assertions: [{ type: 'exit_reason', expected: 'converted' }],
        },
      },
    })
  ).json();
  await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
    headers,
    data: { operation: 'complete', revision: run.revision },
  });
  const current = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  const published = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
    headers,
    data: { revision: current.revision },
  });
  expect(published.ok(), await published.text()).toBeTruthy();

  const total = 24;
  for (let i = 0; i < total; i++) {
    const id = 'shopper_' + String(i).padStart(2, '0');
    await page.request.post('/api/v1/customers', { headers, data: { customer_id: id } });
    expect(
      (
        await page.request.post('/api/v1/events', {
          headers,
          data: {
            event_id: randomBytes(12).toString('hex'),
            customer_id: id,
            event_name: 'cart_updated',
            occurred_at: new Date().toISOString(),
            properties: {},
          },
        })
      ).ok(),
    ).toBeTruthy();
  }
  type Arm = { node_id: string; port: string; entered: number; converted: number };
  const arms = async () =>
    (await (
      await page.request.get(`/api/v1/journeys/${journey.id}/experiments?version=1`)
    ).json()) as Arm[];
  await expect
    .poll(async () => (await arms()).reduce((sum, arm) => sum + arm.entered, 0), {
      timeout: 20000,
    })
    .toBe(total);
  const result = await arms();
  const a = result.find((arm) => arm.port === 'a')!;
  const b = result.find((arm) => arm.port === 'b')!;
  // Both variants get customers, and only variant A leads to the converting exit.
  expect(a.entered).toBeGreaterThan(0);
  expect(b.entered).toBeGreaterThan(0);
  expect(a.converted).toBe(a.entered);
  expect(b.converted).toBe(0);

  // Step and connection counts are projected asynchronously; wait until they have caught up.
  await expect
    .poll(
      async () => {
        const insights = await (
          await page.request.get(`/api/v1/journeys/${journey.id}/insights?version=1&days=30`)
        ).json();
        return (
          insights.metrics as { dimension: string; key: string; metric: string; value: number }[]
        )
          .filter((item) => item.dimension === 'edge' && item.key.startsWith('offer:'))
          .reduce((sum, item) => sum + item.value, 0);
      },
      { timeout: 30000 },
    )
    .toBe(total);
  await page.goto(`/journeys/${journey.id}?tab=analytics`);
  // The path list shows the same traffic: the split step, one row per variant, and the
  // customers on a variant open under its row.
  const paths = page.getByRole('region', { name: 'Customer paths' });
  await expect(paths).toContainText(`${total} customers entered`);
  const variantB = paths.getByRole('button', {
    name: new RegExp(`^Offer test → Variant B: ${b.entered} customers?,`),
  });
  await expect(variantB).toHaveCount(1);
  await variantB.click();
  await expect(paths.locator('.path-person')).toHaveCount(Math.min(6, b.entered));
  const experiments = page.getByRole('region', { name: 'Experiment results' });
  await expect(experiments).toContainText('Offer test');
  await expect(experiments).toContainText('Fewer than 30 customers on a variant');
  const sideA = experiments.locator('.experiment-side').filter({ hasText: 'Variant A' });
  const sideB = experiments.locator('.experiment-side').filter({ hasText: 'Variant B' });
  await expect(sideA.locator('.side-rate')).toHaveText('100.0%');
  await expect(sideB.locator('.side-rate')).toHaveText('0.0%');
  await expect(sideA).toContainText('50% of customers');
  await sideB
    .getByRole('button', { name: `Show ${b.entered} executions on Variant B of Offer test` })
    .click();
  const people = page.locator('.observe-people');
  await expect(people).toContainText('Branch: Variant B');
  await expect(people.locator('.execution-row, tbody tr, .list-row')).toHaveCount(b.entered);
});
