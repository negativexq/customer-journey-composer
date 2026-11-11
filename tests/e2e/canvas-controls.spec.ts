import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';

test('canvas stays light in dark OS mode with readable zoom controls and a wide Elements panel', async ({
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
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/journeys/' + journey.id);
  await expect(page.locator('.vue-flow__node')).toHaveCount(14);
  await expect(page.locator('.canvas > .vue-flow')).toHaveCSS('color-scheme', 'light');
  await expect(page.locator('.canvas > .vue-flow')).toHaveCSS(
    'background-color',
    'rgb(248, 250, 252)',
  );
  const controls = page.locator('.vue-flow__controls');
  await expect(controls).toHaveAttribute('aria-label', 'Canvas controls');
  const zoomIn = controls.locator('.vue-flow__controls-zoomin');
  await expect(zoomIn.locator('svg path')).toHaveAttribute('d', 'M12 5v14M5 12h14');
  await expect(zoomIn.locator('svg')).toHaveCSS('stroke', 'rgb(52, 64, 84)');
  const size = await zoomIn.boundingBox();
  expect(size!.width).toBeGreaterThanOrEqual(38);
  expect(size!.height).toBeGreaterThanOrEqual(38);
  const zoom = controls.getByLabel('Zoom level', { exact: true });
  const before = Number((await zoom.innerText()).replace('%', ''));
  await zoomIn.click();
  await expect
    .poll(async () => Number((await zoom.innerText()).replace('%', '')))
    .toBeGreaterThan(before);
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
});
