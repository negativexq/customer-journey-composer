import { expect, type Locator } from '@playwright/test';

/**
 * Picks a choice in a themed dropdown (AppSelect) or a segmented control, by its name or, for a
 * dropdown, its position.
 */
export async function choose(control: Locator, option: string | RegExp | number) {
  if ((await control.getAttribute('role')) === 'radiogroup') {
    if (typeof option === 'number') await control.getByRole('radio').nth(option).click();
    else
      await control.getByRole('radio', { name: option, exact: typeof option === 'string' }).click();
    return;
  }
  await control.click();
  const page = control.page();
  const target =
    typeof option === 'number'
      ? page.getByRole('option').nth(option)
      : page.getByRole('option', { name: option, exact: typeof option === 'string' });
  await target.click();
  await expect(page.getByRole('listbox')).toHaveCount(0);
}
