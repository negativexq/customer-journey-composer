import { onBeforeUnmount } from 'vue';

// The builder's keyboard shortcuts. They never fire while the person types in a field, and
// only "?" works on a read-only canvas.
export function useCanvasShortcuts(actions: {
  readonly: () => boolean;
  help: () => void;
  selectAll: () => void;
  undo: () => void;
  redo: () => void;
  copy: () => void;
  paste: () => void;
  duplicate: () => void;
}) {
  function keyboard(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)
      return;
    if (
      event.key === '?' &&
      !event.metaKey &&
      !event.ctrlKey &&
      !window.document.querySelector('dialog[open]')
    ) {
      event.preventDefault();
      actions.help();
      return;
    }
    if (actions.readonly() || !(event.metaKey || event.ctrlKey)) return;
    const action = (
      {
        a: actions.selectAll,
        z: event.shiftKey ? actions.redo : actions.undo,
        c: actions.copy,
        v: actions.paste,
        d: actions.duplicate,
      } as Record<string, () => void>
    )[event.key];
    if (!action) return;
    event.preventDefault();
    action();
  }
  window.addEventListener('keydown', keyboard);
  onBeforeUnmount(() => window.removeEventListener('keydown', keyboard));
}
