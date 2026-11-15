import type { Workflow } from './workflow.ts';

// Geometry for a left-to-right flow diagram of one journey version: a bar per step sized by the
// customers who reached it, and a band per connection sized by the customers who took it.
export type FlowNode = {
  id: string;
  label: string;
  type: string;
  value: number;
  column: number;
  x: number;
  y: number;
  height: number;
};
export type FlowLink = {
  id: string;
  source: string;
  target: string;
  port: string;
  value: number;
  // Share of the source step's customers that took this connection.
  share: number;
  path: string;
};
export type FlowLayout = {
  nodes: FlowNode[];
  links: FlowLink[];
  width: number;
  height: number;
  barWidth: number;
  total: number;
};
export type FlowCounts = {
  reached: (nodeID: string) => number;
  taken: (nodeID: string, port: string) => number;
};

const portOrder = (port: string) => {
  if (port === 'false' || port === 'timeout' || port === 'b') return 1;
  if (port === 'else') return 9;
  if (port.startsWith('case_')) return Number(port.slice(5)) - 1;
  return 0;
};

export function pathFlow(
  workflow: Pick<Workflow, 'nodes' | 'edges'>,
  counts: FlowCounts,
  size: { width?: number; height?: number; barWidth?: number; gap?: number } = {},
): FlowLayout {
  const width = size.width ?? 960;
  const height = size.height ?? 320;
  const barWidth = size.barWidth ?? 10;
  const gap = size.gap ?? 18;
  const steps = workflow.nodes.filter((node) => counts.reached(node.id) > 0);
  const present = new Set(steps.map((node) => node.id));
  const edges = workflow.edges
    .filter((edge) => present.has(edge.source) && present.has(edge.target))
    .map((edge) => ({ ...edge, value: counts.taken(edge.source, edge.port) }))
    .filter((edge) => edge.value > 0);
  const empty = { nodes: [], links: [], width, height, barWidth, total: 0 };
  if (!steps.length) return empty;

  // A step sits one column after its furthest predecessor, so every band runs left to right.
  const column = new Map<string, number>();
  const incoming = new Map<string, typeof edges>();
  for (const edge of edges) incoming.set(edge.target, [...(incoming.get(edge.target) || []), edge]);
  const depth = (id: string, seen = new Set<string>()): number => {
    if (column.has(id)) return column.get(id)!;
    if (seen.has(id)) return 0;
    seen.add(id);
    const parents = incoming.get(id) || [];
    const value = parents.length
      ? Math.max(...parents.map((edge) => depth(edge.source, seen))) + 1
      : 0;
    column.set(id, value);
    return value;
  };
  for (const step of steps) depth(step.id);
  const columns = Math.max(...column.values()) + 1;
  const total = Math.max(...steps.map((step) => counts.reached(step.id)));

  // Order within a column follows the order of the bands arriving from the left, which keeps
  // crossings low without a full crossing-minimization pass.
  const byColumn: string[][] = Array.from({ length: columns }, () => []);
  const rank = new Map<string, number>();
  for (let index = 0; index < columns; index++) {
    const ids = steps.filter((step) => column.get(step.id) === index).map((step) => step.id);
    const weight = (id: string) => {
      const parents = incoming.get(id) || [];
      if (!parents.length) return 0;
      return (
        parents.reduce(
          (sum, edge) =>
            sum + ((rank.get(edge.source) || 0) * 10 + portOrder(edge.port)) * edge.value,
          0,
        ) / parents.reduce((sum, edge) => sum + edge.value, 0)
      );
    };
    ids.sort((a, b) => weight(a) - weight(b));
    ids.forEach((id, position) => rank.set(id, position));
    byColumn[index] = ids;
  }
  const tallest = Math.max(
    ...byColumn.map((ids) => ids.reduce((sum, id) => sum + counts.reached(id), 0)),
  );
  const mostRows = Math.max(...byColumn.map((ids) => ids.length));
  const scale = (height - gap * (mostRows - 1)) / tallest;
  const step = columns > 1 ? (width - barWidth) / (columns - 1) : 0;

  const nodes = new Map<string, FlowNode>();
  byColumn.forEach((ids, index) => {
    const used =
      ids.reduce((sum, id) => sum + counts.reached(id) * scale, 0) + gap * (ids.length - 1);
    let y = (height - used) / 2;
    for (const id of ids) {
      const source = workflow.nodes.find((node) => node.id === id)!;
      const value = counts.reached(id);
      const barHeight = Math.max(2, value * scale);
      nodes.set(id, {
        id,
        label: source.label,
        type: source.type,
        value,
        column: index,
        x: index * step,
        y,
        height: barHeight,
      });
      y += barHeight + gap;
    }
  });

  // Bands leave a bar in output order and arrive in the order of their sources.
  const leaving = new Map<string, number>();
  const arriving = new Map<string, number>();
  const ordered = [...edges].sort(
    (a, b) =>
      nodes.get(a.source)!.column - nodes.get(b.source)!.column ||
      nodes.get(a.source)!.y - nodes.get(b.source)!.y ||
      portOrder(a.port) - portOrder(b.port),
  );
  const links = ordered.map((edge): FlowLink => {
    const from = nodes.get(edge.source)!;
    const to = nodes.get(edge.target)!;
    const thickness = Math.max(1, edge.value * scale);
    const top = from.y + (leaving.get(from.id) || 0);
    const bottom = to.y + (arriving.get(to.id) || 0);
    leaving.set(from.id, (leaving.get(from.id) || 0) + thickness);
    arriving.set(to.id, (arriving.get(to.id) || 0) + thickness);
    const x0 = from.x + barWidth;
    const x1 = to.x;
    const bend = (x0 + x1) / 2;
    const n = (value: number) => Math.round(value * 10) / 10;
    return {
      id: edge.id,
      source: edge.source,
      target: edge.target,
      port: edge.port,
      value: edge.value,
      share: from.value ? edge.value / from.value : 0,
      path:
        `M${n(x0)},${n(top)} C${n(bend)},${n(top)} ${n(bend)},${n(bottom)} ${n(x1)},${n(bottom)} ` +
        `L${n(x1)},${n(bottom + thickness)} C${n(bend)},${n(bottom + thickness)} ${n(bend)},${n(top + thickness)} ${n(x0)},${n(top + thickness)} Z`,
    };
  });
  return { nodes: [...nodes.values()], links, width, height, barWidth, total };
}
