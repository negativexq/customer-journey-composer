import dagre from '@dagrejs/dagre';
import type { Workflow } from './workflow';
const width = 248,
  height = 220,
  gap = 88;
// Geometry belongs to the editor, not the model. Preserve hand-arranged existing
// steps during modification; only new nodes get generated positions.
export function layoutGeneratedWorkflow(source: Workflow, previous?: Workflow): Workflow {
  const result: Workflow = JSON.parse(JSON.stringify(source));
  const graph = new dagre.graphlib.Graph({ multigraph: true });
  graph.setGraph({ rankdir: 'TB', nodesep: 100, ranksep: gap });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const node of result.nodes) graph.setNode(node.id, { width, height });
  for (const edge of result.edges) graph.setEdge(edge.source, edge.target, {}, edge.id);
  dagre.layout(graph);
  const existing = new Map(previous?.nodes.map((n) => [n.id, n.position]) || []);
  const placed: { x: number; y: number }[] = [];
  for (const node of result.nodes) {
    const saved = existing.get(node.id);
    if (saved) {
      node.position = { ...saved };
      placed.push(node.position);
    }
  }
  for (const node of result.nodes) {
    if (existing.has(node.id)) continue;
    const p = graph.node(node.id);
    const candidate = { x: p.x - width / 2, y: p.y - height / 2 };
    while (
      placed.some(
        (q) =>
          Math.abs(q.x - candidate.x) < width + 24 && Math.abs(q.y - candidate.y) < height + 24,
      )
    )
      candidate.x += width + 100;
    node.position = candidate;
    placed.push(candidate);
  }
  return result;
}
