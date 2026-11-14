import { reactive, ref } from 'vue';

// Short confirmations for actions that already succeeded. Errors stay inline, next to what failed.
export type Toast = { id: number; message: string };
export const toasts = reactive<Toast[]>([]);
// The latest message that no inline status already announces to assistive technology.
export const announcement = ref('');
const timers = new Map<number, ReturnType<typeof setTimeout>>();
const DURATION = 4500;
let nextID = 1;

export function dismissToast(id: number) {
  clearTimeout(timers.get(id));
  timers.delete(id);
  const index = toasts.findIndex((item) => item.id === id);
  if (index >= 0) toasts.splice(index, 1);
}
export function holdToast(id: number) {
  clearTimeout(timers.get(id));
}
export function releaseToast(id: number) {
  holdToast(id);
  timers.set(
    id,
    setTimeout(() => dismissToast(id), DURATION),
  );
}
// Pass `announce: false` when the screen already reports the outcome in a status region.
export function toast(message: string, options: { announce?: boolean } = {}) {
  const repeated = toasts.find((item) => item.message === message);
  const id = repeated?.id ?? nextID++;
  if (!repeated) toasts.push({ id, message });
  while (toasts.length > 3) dismissToast(toasts[0]!.id);
  if (options.announce !== false) announcement.value = message;
  releaseToast(id);
}
