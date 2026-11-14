import { ref } from 'vue';

// 'system' follows the device; 'light' and 'dark' pin the choice on this browser.
export type Theme = 'system' | 'light' | 'dark';
const KEY = 'journey-composer.theme';
const order: Theme[] = ['system', 'light', 'dark'];
export const themeLabel: Record<Theme, string> = {
  system: 'Match system',
  light: 'Light',
  dark: 'Dark',
};

function stored(): Theme {
  try {
    const value = localStorage.getItem(KEY);
    return value === 'light' || value === 'dark' ? value : 'system';
  } catch {
    return 'system';
  }
}
export const theme = ref<Theme>(stored());

export function applyTheme(next: Theme) {
  theme.value = next;
  const root = document.documentElement;
  if (next === 'system') delete root.dataset.theme;
  else root.dataset.theme = next;
  try {
    if (next === 'system') localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, next);
  } catch {
    // Private browsing can refuse storage; the choice still applies to this page.
  }
}
export const nextTheme = (current: Theme): Theme =>
  order[(order.indexOf(current) + 1) % order.length]!;
