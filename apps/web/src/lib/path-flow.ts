import type { Workflow } from './workflow.ts';
import { portLabel } from './workflow-presentation.ts';

// Where customers went, as a list that reads like the journey: one row per step in workflow
// order, branch rows under a step that splits, and rows for customers who stopped at a step.
export type PathRow =
  | {
      kind: 'step';
      key: string;
      depth: number;
      node: string;
      label: string;
      type: string;
      value: number;
    }
  | {
      kind: 'branch';
      key: string;
      depth: number;
      node: string;
      port: string;
      label: string;
      value: number;
    }
  | {
      kind: 'waiting' | 'left';
      key: string;
      depth: number;
      node: string;
      label: string;
      value: number;
    }
  | { kind: 'joins'; key: string; depth: number; node: string; label: string; value: number };
export type PathCounts = {
  reached: (nodeID: string) => number;
  taken: (nodeID: string, port: string) => number;
  waiting: (nodeID: string) => number;
};
export type PathSummary = {
  entered: number;
  finished: number;
  // The step where the most customers stopped without continuing, if any did.
  biggestLoss?: { node: string; label: string; value: number };
};

const portOrder = (port: string) => {
  if (port === 'true' || port === 'matched' || port === 'a') return 0;
  if (port === 'false' || port === 'timeout' || port === 'b') return 1;
  if (port === 'else') return 99;
  if (port.startsWith('case_')) return Number(port.slice(5)) + 1;
  return 50;
};

export function pathRows(
  workflow: Pick<Workflow, 'nodes' | 'edges'>,
  counts: PathCounts,
): { rows: PathRow[]; summary: PathSummary } {
  const byId = new Map(workflow.nodes.map((node) => [node.id, node]));
  const targets = new Set(workflow.edges.map((edge) => edge.target));
  const sources = new Set(workflow.edges.map((edge) => edge.source));
  // Entry steps start the journey: nothing leads to them and they lead somewhere, or they are
  // a trigger on their own. A stray unconnected step is not an entry.
  const entries = workflow.nodes.filter(
    (node) => !targets.has(node.id) && (sources.has(node.id) || node.type.endsWith('trigger')),
  );
  const rows: PathRow[] = [];
  const seen = new Set<string>();
  const losses: { node: string; label: string; value: number }[] = [];
  let finished = 0;

  const visit = (id: string, depth: number) => {
    const node = byId.get(id);
    if (!node) return;
    const value = counts.reached(id);
    if (seen.has(id)) {
      // A step reached by more than one path is listed once; later paths point back to it.
      rows.push({
        kind: 'joins',
        key: `joins:${id}:${rows.length}`,
        depth,
        node: id,
        label: node.label,
        value,
      });
      return;
    }
    seen.add(id);
    rows.push({
      kind: 'step',
      key: `step:${id}`,
      depth,
      node: id,
      label: node.label,
      type: node.type,
      value,
    });
    const out = workflow.edges
      .filter((edge) => edge.source === id)
      .sort((a, b) => portOrder(a.port) - portOrder(b.port));
    if (!out.length) {
      if (node.type === 'exit') finished += value;
    }
    const took = out.reduce((sum, edge) => sum + counts.taken(id, edge.port), 0);
    const waiting = Math.min(counts.waiting(id), Math.max(0, value - took));
    const left = node.type === 'exit' ? 0 : Math.max(0, value - took - waiting);
    const stops = () => {
      if (waiting)
        rows.push({
          kind: 'waiting',
          key: `waiting:${id}`,
          depth: depth + 1,
          node: id,
          label: 'Waiting here now',
          value: waiting,
        });
      if (left) {
        rows.push({
          kind: 'left',
          key: `left:${id}`,
          depth: depth + 1,
          node: id,
          label: 'Left the journey here',
          value: left,
        });
        losses.push({ node: id, label: node.label, value: left });
      }
    };
    if (out.length > 1) {
      for (const edge of out) {
        rows.push({
          kind: 'branch',
          key: `branch:${id}:${edge.port}`,
          depth: depth + 1,
          node: id,
          port: edge.port,
          label: portLabel(edge.port),
          value: counts.taken(id, edge.port),
        });
        visit(edge.target, depth + 2);
      }
      stops();
    } else {
      stops();
      if (out[0]) visit(out[0].target, depth);
    }
  };
  for (const entry of entries) visit(entry.id, 0);

  const entered = entries.reduce((sum, node) => sum + counts.reached(node.id), 0);
  const biggestLoss = losses.sort((a, b) => b.value - a.value)[0];
  return { rows, summary: { entered, finished, ...(biggestLoss ? { biggestLoss } : {}) } };
}
