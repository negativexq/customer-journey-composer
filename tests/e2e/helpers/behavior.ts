import type { Page } from '@playwright/test';
export async function confirmEmptyBehavior(page: Page) {
  const region = page.getByRole('region', { name: 'Independent behavior checks', exact: true });
  await region.getByRole('combobox', { name: 'Expected entry', exact: true }).click();
  await page.getByRole('option').nth(1).click();
  await region.getByLabel('Expected emails', { exact: true }).fill('0');
  await region.getByLabel('Expected SMS', { exact: true }).fill('0');
  await region
    .getByLabel('I reviewed these expectations and the synthetic example.', { exact: true })
    .check();
}
export function workflowContent(w: Record<string, unknown>) {
  // Geometry and independently persisted checks are tested separately.
  const { behavior_checks: _checks, ...content } = w;
  void _checks;
  return {
    ...content,
    nodes: (w.nodes as Record<string, unknown>[]).map(({ position: _position, ...node }) => {
      void _position;
      return node;
    }),
  };
}
