import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('canvas follows the theme with readable zoom controls and a wide Elements panel', async ({
  page,
}) => {
  const user = await (
    await page.request.post('/api/v1/auth/register', {
      data: {
        name: 'Canvas reviewer',
        workspace: 'Canvas controls',
        email: randomBytes(12).toString('hex') + '@example.invalid',
        password: randomBytes(24).toString('hex'),
      },
    })
  ).json();
  const journey = await (
    await page.request.post('/api/v1/templates/abandoned-cart-eur/create-journey', {
      headers: { 'X-CSRF-Token': user.csrf_token },
      data: {},
    })
  ).json();
  await page.goto('/journeys/' + journey.id);
  await expect(page.locator('.vue-flow__node')).toHaveCount(14);
  const flow = page.locator('.canvas > .vue-flow');
  // Relative luminance of a computed color, so the assertion holds whatever the exact palette is.
  const luminance = (locator: typeof flow, property: string) =>
    locator.evaluate((element, name) => {
      const context = document.createElement('canvas').getContext('2d')!;
      context.fillStyle = getComputedStyle(element).getPropertyValue(name);
      context.fillRect(0, 0, 1, 1);
      const [r, g, b] = [...context.getImageData(0, 0, 1, 1).data].map((value) => {
        const channel = value / 255;
        return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
      });
      return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
    }, property);
  const node = page.locator('.workflow-node').first();
  await expect.poll(() => luminance(flow, 'background-color')).toBeGreaterThan(0.8);
  await expect.poll(() => luminance(node, 'background-color')).toBeGreaterThan(0.8);
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect.poll(() => luminance(flow, 'background-color')).toBeLessThan(0.05);
  await expect.poll(() => luminance(node, 'background-color')).toBeLessThan(0.05);
  await expect
    .poll(() => luminance(page.locator('.node-title').first(), 'color'))
    .toBeGreaterThan(0.5);
  // One floating bar holds the canvas tools; the old toolbar row above the canvas is gone.
  const controls = page.getByRole('toolbar', { name: 'Canvas controls', exact: true });
  await expect(page.locator('.builder-toolbar')).toHaveCount(0);
  const mobile = test.info().project.name === 'mobile';
  // Every tool in the bar is a readable, finger-sized target on the themed surface.
  const fit = controls.getByRole('button', { name: 'Fit view', exact: true });
  await expect.poll(() => luminance(fit.locator('svg'), 'stroke')).toBeGreaterThan(0.5);
  await expect.poll(() => luminance(fit, 'background-color')).toBeLessThan(0.05);
  const size = await fit.boundingBox();
  expect(size!.width).toBeGreaterThanOrEqual(38);
  expect(size!.height).toBeGreaterThanOrEqual(38);
  const bar = await controls.boundingBox();
  const canvas = await page.locator('.canvas').boundingBox();
  expect(bar!.x).toBeGreaterThanOrEqual(canvas!.x);
  expect(bar!.x + bar!.width).toBeLessThanOrEqual(canvas!.x + canvas!.width);
  const zoomIn = controls.getByRole('button', { name: 'Zoom in', exact: true });
  if (mobile) {
    // Phones pinch to zoom; the buttons give their room to tools without a gesture.
    await expect(zoomIn).toBeHidden();
  } else {
    await expect(zoomIn.locator('svg path')).toHaveAttribute('d', 'M12 5v14M5 12h14');
    const zoom = controls.getByLabel('Zoom level', { exact: true });
    const before = Number((await zoom.innerText()).replace('%', ''));
    await zoomIn.click();
    await expect
      .poll(async () => Number((await zoom.innerText()).replace('%', '')))
      .toBeGreaterThan(before);
  }
  const elements = page.getByRole('button', { name: 'Toggle step library', exact: true });
  if ((await elements.getAttribute('aria-expanded')) === 'true') await elements.click();
  const canvasBefore = await page.locator('.canvas').boundingBox();
  await elements.click();
  const library = page.getByRole('complementary', { name: 'Step library', exact: true });
  const width = await library.boundingBox();
  expect(width!.width).toBeGreaterThanOrEqual(220);
  if (test.info().project.name === 'mobile') {
    const canvasAfter = await page.locator('.canvas').boundingBox();
    expect(Math.abs(canvasAfter!.width - canvasBefore!.width)).toBeLessThan(1);
  }
  await expect(
    library.getByRole('button', { name: '+ Update Attribute', exact: true }),
  ).toBeVisible();

  const shortcuts = page.getByRole('dialog', { name: 'Keyboard shortcuts', exact: true });
  await page.getByLabel('More canvas actions', { exact: true }).click();
  await page.getByRole('menuitem', { name: 'Keyboard shortcuts', exact: true }).click();
  await expect(shortcuts).toContainText('Duplicate selected steps');
  await page.keyboard.press('Escape');
  await expect(shortcuts).toBeHidden();
  // The dialog reports its close on a later task; retry so the key is not pressed in between.
  await expect(async () => {
    await page.keyboard.press('?');
    await expect(shortcuts).toBeVisible({ timeout: 1000 });
  }).toPass();
});
