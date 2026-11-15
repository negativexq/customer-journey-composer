import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('versions explain unpublished and published changes in product language', async ({ page }) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Reviewer',
        workspace: 'Versions',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const headers = { 'X-CSRF-Token': user.csrf_token };
  const journey = await (
    await page.request.post('/api/v1/journeys', { headers, data: { name: 'Versioned journey' } })
  ).json();
  const spec = (seconds: number, position = 180) => ({
    schema_version: 1,
    nodes: [
      {
        id: 'start',
        type: 'event_trigger',
        label: 'Start',
        position: { x: 0, y: 0 },
        config: { event_name: 'cart_updated' },
      },
      {
        id: 'delay',
        type: 'wait',
        label: 'Pause',
        position: { x: 0, y: position },
        config: { duration_seconds: seconds },
      },
      {
        id: 'done',
        type: 'exit',
        label: 'Done',
        position: { x: 0, y: 360 },
        config: { reason: 'completed' },
      },
    ],
    edges: [
      { id: 'a', source: 'start', target: 'delay', port: 'started' },
      { id: 'b', source: 'delay', target: 'done', port: 'completed' },
    ],
  });
  const current = async () =>
    (await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json()) as {
      revision: number;
    };
  const saveDraft = async (draft: unknown) => {
    const response = await page.request.put(`/api/v1/journeys/${journey.id}/draft`, {
      headers,
      data: { name: journey.name, draft, revision: (await current()).revision },
    });
    expect(response.ok()).toBeTruthy();
  };
  const publish = async () => {
    const gate = await (
      await page.request.post('/api/v1/test-runs', {
        headers,
        data: {
          journey_id: journey.id,
          scenario: { name: 'Version gate', entry_event: { event_name: 'cart_updated' } },
        },
      })
    ).json();
    await page.request.post(`/api/v1/test-runs/${gate.id}/advance`, {
      headers,
      data: { operation: 'complete', revision: gate.revision },
    });
    const response = await page.request.post(`/api/v1/journeys/${journey.id}/publish`, {
      headers,
      data: { revision: (await current()).revision },
    });
    expect(response.ok()).toBeTruthy();
  };

  await saveDraft(spec(3));
  await publish();
  // Moving a step is layout, not behavior.
  await saveDraft(spec(3, 240));
  await page.goto(`/journeys/${journey.id}?tab=versions`);
  await expect(
    page.getByRole('heading', { name: 'Published versions', exact: true }),
  ).toBeVisible();
  const draftChanges = page.getByTestId('draft-changes');
  await expect(draftChanges).toContainText('Draft matches v1');

  const versionOne = page.getByRole('button', { name: /Version 1/ });
  await expect(versionOne).toContainText('Receiving new entries');
  await versionOne.click();
  await expect(versionOne).toHaveAttribute('aria-expanded', 'true');
  const detail = page.locator('.version-detail');
  await expect(detail).toContainText('3 steps · 2 connections');
  await expect(detail.locator('pre')).toHaveCount(0);
  await detail.getByRole('button', { name: 'Show technical definition' }).click();
  await expect(detail.locator('pre')).toContainText('"schema_version": 1');

  await saveDraft(spec(120));
  await page.reload();
  await expect(draftChanges).toContainText('Draft has 1 unpublished change since v1');
  await expect(draftChanges).toContainText('Wait 3 seconds → Wait 2 minutes');

  await publish();
  await page.reload();
  await expect(draftChanges).toContainText('Draft matches v2');
  await expect(page.getByRole('button', { name: /Version 2/ })).toContainText(
    'Receiving new entries',
  );
  await expect(page.getByRole('button', { name: /Version 1/ })).not.toContainText(
    'Receiving new entries',
  );
  await page.getByRole('button', { name: 'Compare versions', exact: true }).click();
  const versionChanges = page.getByTestId('version-changes');
  await expect(versionChanges).toContainText('v1 → v2 · 1 change');
  await expect(versionChanges).toContainText('Wait 3 seconds → Wait 2 minutes');

  // An old version can be put back on the draft; customers see nothing until it is published.
  await page.getByRole('button', { name: /Version 1/ }).click();
  await page.getByRole('button', { name: 'Restore to draft', exact: true }).click();
  const confirm = page.getByRole('dialog', { name: 'Restore version 1 to the draft?' });
  await expect(confirm).toContainText('Customers are not affected until you publish');
  await confirm.getByRole('button', { name: 'Restore to draft', exact: true }).click();
  await expect(page.getByTestId('toast')).toContainText('Version 1 restored to the draft');
  await expect(draftChanges).toContainText('Draft has 1 unpublished change since v2');
  await expect(draftChanges).toContainText('Wait 2 minutes → Wait 3 seconds');
  const restored = await (await page.request.get(`/api/v1/journeys/${journey.id}`)).json();
  expect(
    restored.draft.nodes.find((node: { id: string }) => node.id === 'delay').config
      .duration_seconds,
  ).toBe(3);
  expect(restored.version).toBe(2);

  // The publish review states the same change before anything goes live.
  await page.getByRole('button', { name: 'Publish', exact: true }).click();
  const review = page.getByRole('region', { name: 'What this publish changes' });
  await expect(review).toContainText('1 change since version 2');
  await expect(review).toContainText('Wait 2 minutes → Wait 3 seconds');
});
