import type { Ref } from 'vue';
import type { CanvasEdge, CanvasNode } from '../lib/step-catalog';

// Copy and paste of steps inside the builder. A copy keeps the connections between the copied
// steps; pasted steps get new IDs and land a little below and to the right.
export function useCanvasClipboard(options: {
  nodes: Ref<CanvasNode[]>;
  edges: Ref<CanvasEdge[]>;
  selectedID: Ref<string>;
  chosen: () => CanvasNode[];
  readonly: () => boolean;
  overLimit: () => void;
}) {
  const { nodes, edges, selectedID } = options;
  let clipboard = '';
  function copy() {
    const chosen = options.chosen();
    const ids = new Set(chosen.map((n) => n.id));
    clipboard = JSON.stringify({
      nodes: chosen,
      edges: edges.value.filter((e) => ids.has(e.source) && ids.has(e.target)),
    });
  }
  function paste() {
    if (!clipboard || options.readonly()) return;
    const copied = JSON.parse(clipboard) as { nodes: CanvasNode[]; edges: CanvasEdge[] };
    if (
      nodes.value.length + copied.nodes.length > 100 ||
      edges.value.length + copied.edges.length > 200
    ) {
      options.overLimit();
      return;
    }
    const ids = new Map<string, string>();
    for (const n of copied.nodes) ids.set(n.id, crypto.randomUUID());
    const added = copied.nodes.map((n) => ({
      ...n,
      id: ids.get(n.id)!,
      position: { x: n.position.x + 40, y: n.position.y + 40 },
      selected: true,
    }));
    nodes.value = [...nodes.value.map((n) => ({ ...n, selected: false })), ...added];
    edges.value = [
      ...edges.value,
      ...copied.edges.map((e) => ({
        ...e,
        id: crypto.randomUUID(),
        source: ids.get(e.source)!,
        target: ids.get(e.target)!,
      })),
    ];
    selectedID.value = added[0]?.id || '';
  }
  function duplicate() {
    copy();
    paste();
  }
  return { copy, paste, duplicate };
}
