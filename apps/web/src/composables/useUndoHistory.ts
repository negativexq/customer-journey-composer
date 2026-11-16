import { nextTick, watch } from 'vue';

// Undo and redo over snapshots of a document. Every change to the snapshot is recorded;
// restoring one puts it back without recording it again.
export function useUndoHistory(
  snapshot: () => string,
  restore: (data: string) => void,
  changed: (data: string) => void,
  limit = 60,
) {
  const history: string[] = [];
  const future: string[] = [];
  let restoring = false;
  let previous = snapshot();
  watch(snapshot, (current) => {
    if (restoring) {
      previous = current;
      return;
    }
    if (previous === current) return;
    history.push(previous);
    if (history.length > limit) history.shift();
    future.length = 0;
    previous = current;
    changed(current);
  });
  async function apply(data: string) {
    restoring = true;
    restore(data);
    await nextTick();
    restoring = false;
    previous = data;
    changed(data);
  }
  function undo() {
    const old = history.pop();
    if (old) {
      future.push(previous);
      void apply(old);
    }
  }
  function redo() {
    const next = future.pop();
    if (next) {
      history.push(previous);
      void apply(next);
    }
  }
  return { undo, redo };
}
