import { expect, type Locator } from '@playwright/test';

/** Opens a themed dropdown (AppSelect) and picks an option by its name or position. */
export async function choose(trigger: Locator, option: string | RegExp | number) {
  await trigger.click();
  const page = trigger.page();
  const target =
    typeof option === 'number'
      ? page.getByRole('option').nth(option)
      : page.getByRole('option', { name: option, exact: typeof option === 'string' });
  await target.click();
  await expect(page.getByRole('listbox')).toHaveCount(0);
}
