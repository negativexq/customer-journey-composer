import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('a journey is built from the canvas and reads as a sentence', async ({ page }) => {
  const identity = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Builder',
        workspace: 'Canvas add',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  await page.request.post('/api/v1/journeys', {
    headers: { 'X-CSRF-Token': identity.csrf_token },
    data: { name: 'Built on canvas' },
  });
  await page.goto('/');
  await page.locator('.journey-name-button').filter({ hasText: 'Built on canvas' }).click();

  const nodes = page.locator('.canvas .vue-flow__node');
  const picker = page.getByRole('dialog', { name: 'Add a step', exact: true });
  const choose = (kind: string) =>
    picker.getByRole('button', { name: new RegExp('^' + kind) }).click();
  // Adding selects the new step, which dims the rest; clicking empty canvas clears the selection.
  const clear = () => page.locator('.canvas .vue-flow__pane').click({ position: { x: 6, y: 150 } });

  // The step library stays closed; the empty canvas offers the first step itself.
  await expect(page.getByRole('complementary', { name: 'Step library' })).toBeHidden();
  await page.getByRole('button', { name: 'Add a trigger', exact: true }).click();
  await expect(nodes).toHaveCount(1);
  await expect(nodes.first().locator('.node-caption')).toHaveText('Starts when');
  const settings = page.getByRole('complementary', { name: 'Step settings' });
  if (await settings.isVisible())
    await expect(settings).toContainText('A customer enters the journey each time this event');
  await clear();

  await page.getByRole('button', { name: 'Add a step after Event Trigger', exact: true }).click();
  await expect(picker).toContainText('After Event Trigger');
  // A journey has one entry, so the picker does not offer another trigger.
  await expect(picker.getByRole('button', { name: /^Event Trigger/ })).toHaveCount(0);
  await choose('Wait for Event');
  await expect(nodes).toHaveCount(2);
  await expect(page.locator('.canvas .vue-flow__edge')).toHaveCount(1);
  await clear();

  await page
    .getByRole('button', { name: 'Add a step after Wait for Event · Timeout', exact: true })
    .click();
  await expect(picker).toContainText('After Wait for Event · Timeout');
  await choose('Send SMS');
  await clear();
  await page
    .getByRole('button', { name: 'Add a step after Wait for Event · Matched', exact: true })
    .click();
  await choose('Exit');
  await clear();
  await expect(nodes).toHaveCount(4);
  await expect(page.locator('.canvas .vue-flow__edge')).toHaveCount(3);

  const sms = nodes.filter({ hasText: 'Send SMS' });
  const exit = nodes.filter({ hasText: 'Exit' });
  await expect(sms.locator('.node-caption')).toHaveText(/^Otherwise, after /);
  await expect(exit.locator('.node-caption')).toHaveText(/^If .+ arrives$/);
  // Matched leaves from the left handle and Timeout from the right, so their steps sit that way.
  const [left, right] = [await exit.boundingBox(), await sms.boundingBox()];
  expect(left!.x).toBeLessThan(right!.x);
  // Branch names are stated once, on the arriving step, not repeated inside the card.
  await expect(page.locator('.canvas .node-ports')).toHaveCount(0);
  // Only the SMS step still has an open output.
  await expect(page.getByRole('button', { name: /^Add a step after / })).toHaveCount(1);

  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(nodes).toHaveCount(3);
  await expect(
    page.getByRole('button', { name: 'Add a step after Wait for Event · Matched', exact: true }),
  ).toBeVisible();
});
