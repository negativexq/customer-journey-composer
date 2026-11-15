import dagre from '@dagrejs/dagre';

// Top-to-bottom layout for a journey graph. dagre knows nothing about ports, so on its own it can
// put the "Timeout" branch under the left handle and "Matched" under the right one, and the two
// connections cross. Branch targets are kept in the order their ports appear on the card.
export type LayoutNode = { id: string; width: number; height: number };
export type LayoutEdge = { id: string; source: string; target: string; port?: string };
export type LayoutOptions = { nodesep?: number; ranksep?: number; margin?: number };

// Left-to-right position of an output handle on its card.
const portRank = (port?: string) => {
  if (port === 'false' || port === 'timeout' || port === 'b') return 1;
  if (port === 'else') return 9;
  if (port?.startsWith('case_')) return Number(port.slice(5)) - 1;
  return 0;
};

export function layoutFlow(
  nodes: LayoutNode[],
  edges: LayoutEdge[],
  options: LayoutOptions = {},
): Record<string, { x: number; y: number }> {
  const graph = new dagre.graphlib.Graph({ multigraph: true });
  graph.setGraph({
    rankdir: 'TB',
    nodesep: options.nodesep ?? 100,
    ranksep: options.ranksep ?? 88,
    marginx: options.margin ?? 0,
    marginy: options.margin ?? 0,
  });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const node of nodes) graph.setNode(node.id, { width: node.width, height: node.height });
  // dagre seeds its ordering from insertion order, so offering branches left-first is usually enough.
  const ordered = edges
    .map((edge, index) => ({ edge, index }))
    .sort((a, b) => portRank(a.edge.port) - portRank(b.edge.port) || a.index - b.index)
    .map((item) => item.edge);
  for (const edge of ordered) graph.setEdge(edge.source, edge.target, {}, edge.id);
  dagre.layout(graph);

  const centers = new Map(
    nodes.map((node) => {
      const placed = graph.node(node.id);
      return [node.id, { x: placed.x, y: placed.y }];
    }),
  );
  const children = new Map<string, string[]>();
  for (const edge of edges)
    children.set(edge.source, [...(children.get(edge.source) || []), edge.target]);
  const reach = (start: string) => {
    const seen = new Set<string>();
    const queue = [start];
    while (queue.length) {
      const id = queue.pop()!;
      if (seen.has(id)) continue;
      seen.add(id);
      queue.push(...(children.get(id) || []));
    }
    return seen;
  };
  // Where the seed was not enough, exchange the two branches' columns. Steps both branches lead
  // to stay where they are.
  for (const node of nodes) {
    const out = edges.filter((edge) => edge.source === node.id);
    const left = out.find((edge) => portRank(edge.port) === 0);
    const right = out.find((edge) => portRank(edge.port) === 1);
    if (!left || !right || left.target === right.target) continue;
    const a = centers.get(left.target);
    const b = centers.get(right.target);
    if (!a || !b || a.x <= b.x) continue;
    const leftTree = reach(left.target);
    const rightTree = reach(right.target);
    if (leftTree.has(node.id) || rightTree.has(node.id)) continue;
    const delta = a.x - b.x;
    for (const id of leftTree) if (!rightTree.has(id)) centers.get(id)!.x -= delta;
    for (const id of rightTree) if (!leftTree.has(id)) centers.get(id)!.x += delta;
  }
  return Object.fromEntries(
    nodes.map((node) => {
      const center = centers.get(node.id)!;
      return [node.id, { x: center.x - node.width / 2, y: center.y - node.height / 2 }];
    }),
  );
}
