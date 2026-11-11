import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('failed required cases block publishing and open Test Lab for correction', async ({
  page,
}) => {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Acceptance editor',
        workspace: 'Required case review',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': identity.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Acceptance review' } })
  ).json();
  expect(
    (
      await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
        headers,
        data: {
          name: journey.name,
          revision: journey.revision,
          draft: {
            schema_version: 1,
            nodes: [
              {
                id: 'start',
                type: 'event_trigger',
                label: 'Cart',
                position: { x: 0, y: 0 },
                config: { event_name: 'cart_updated' },
              },
              {
                id: 'done',
                type: 'exit',
                label: 'Done',
                position: { x: 0, y: 180 },
                config: { reason: 'completed' },
              },
            ],
            edges: [{ id: 'a', source: 'start', target: 'done', port: 'started' }],
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  const scene = (name: string, expected: string) => ({
    name,
    entry_event: { event_name: 'cart_updated' },
    assertions: [{ type: 'state', expected }],
  });
  const saved = await (
    await page.request.post('/api/v1/test-scenarios', {
      headers,
      data: { journey_id: journey.id, scenario: scene('Required conversion', 'EXITED') },
    })
  ).json();
  for (const input of [
    { journey_id: journey.id, scenario: scene('Smoke', 'COMPLETED') },
    { scenario_id: saved.id },
  ]) {
    const run = await (
      await page.request.post('/api/v1/test-runs', { headers, data: input })
    ).json();
    expect(
      (
        await page.request.post(`/api/v1/test-runs/${run.id}/advance`, {
          headers,
          data: { operation: 'complete', revision: run.revision },
        })
      ).ok(),
    ).toBeTruthy();
  }
  await page.goto(`/journeys/${journey.id}`);
  await page.getByRole('button', { name: 'Publish', exact: true }).click();
  const review = page.getByRole('dialog', { name: 'Review before publishing' });
  await expect(review).toContainText('acceptance scenarios failed');
  await expect(review.getByRole('button', { name: 'Publish version', exact: true })).toBeDisabled();
  await review.getByRole('button', { name: 'Open Test Lab', exact: false }).click();
  await expect(page.locator('.test-lab')).toBeVisible();
  // Optionality is an explicit human action; the passing smoke alone did not hide this case.
  expect(
    (
      await page.request.patch(`/api/v1/test-scenarios/${saved.id}/requirement`, {
        headers,
        data: { required: false },
      })
    ).ok(),
  ).toBeTruthy();
  expect(
    (await (await page.request.get(`/api/v1/journeys/${journey.id}/publish-check`)).json()).ready,
  ).toBe(true);
  expect(await (await page.request.get(`/api/v1/journeys/${journey.id}/versions`)).json()).toEqual(
    [],
  );
});
