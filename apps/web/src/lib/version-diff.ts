import type { Step, Workflow } from './workflow';

export type StepChange = { step: Step; changes: string[] };
export type WorkflowDiff = {
  added: Step[];
  removed: Step[];
  changed: StepChange[];
  connectionsChanged: boolean;
  exitChanged: boolean;
  reentryChanged: boolean;
  /** Number of behavior-relevant differences; canvas positions are ignored. */
  total: number;
};

// Key order must not make two equal definitions look different.
function stable(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stable).join(',')}]`;
  if (value && typeof value === 'object')
    return `{${Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([k, v]) => JSON.stringify(k) + ':' + stable(v))
      .join(',')}}`;
  return JSON.stringify(value ?? null);
}

const connections = (workflow: Workflow) =>
  workflow.edges.map((edge) => `${edge.source}|${edge.port}|${edge.target}`).sort();

/**
 * Compare two workflow definitions the way a reviewer reads them: steps added,
 * removed or reconfigured, plus routing and journey-level rules. Moving a step
 * on the canvas is not a behavior change and is not reported.
 */
export function diffWorkflows(
  before: Workflow,
  after: Workflow,
  summarize: (step: Step) => string,
  typeLabel: (step: Step) => string = (step) => step.type,
): WorkflowDiff {
  const old = new Map(before.nodes.map((node) => [node.id, node]));
  const current = new Map(after.nodes.map((node) => [node.id, node]));
  const changed: StepChange[] = [];
  for (const step of after.nodes) {
    const previous = old.get(step.id);
    if (!previous) continue;
    const changes: string[] = [];
    if (previous.label !== step.label) changes.push(`Renamed from “${previous.label}”`);
    if (previous.type !== step.type)
      changes.push(`Type changed from ${typeLabel(previous)} to ${typeLabel(step)}`);
    if (stable(previous.config) !== stable(step.config)) {
      const was = summarize(previous);
      const now = summarize(step);
      changes.push(was === now ? 'Settings or content edited' : `${was} → ${now}`);
    }
    if (changes.length) changed.push({ step, changes });
  }
  const added = after.nodes.filter((node) => !old.has(node.id));
  const removed = before.nodes.filter((node) => !current.has(node.id));
  const connectionsChanged = stable(connections(before)) !== stable(connections(after));
  const exitChanged = stable(before.global_exit) !== stable(after.global_exit);
  const reentryChanged = stable(before.reentry) !== stable(after.reentry);
  return {
    added,
    removed,
    changed,
    connectionsChanged,
    exitChanged,
    reentryChanged,
    total:
      added.length +
      removed.length +
      changed.length +
      Number(connectionsChanged) +
      Number(exitChanged) +
      Number(reentryChanged),
  };
}
