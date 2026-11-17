// A sandbox run as Test Lab shows it: its virtual clock, steps, messages and checks.
export type TraceEntry = { at: string; kind: string; node_id?: string; message: string };
export type RunState = {
  now: string;
  status: string;
  result: string;
  paused: boolean;
  finalized: boolean;
  current_node_id?: string;
  exit_reason?: string;
  trace: TraceEntry[];
  steps: {
    node_id: string;
    label: string;
    state: string;
    started_at: string;
    deadline?: string;
    output: unknown;
  }[];
  messages: {
    id: string;
    channel: string;
    status: string;
    scheduled_at: string;
    decision: unknown;
  }[];
  assertions: { description: string; pass: boolean; actual: unknown }[];
};
export type SandboxRun = { id: string; revision: number; workflow_hash: string; state: RunState };

// Sandbox times are virtual and shown in UTC.
export const sandboxTime = (at: string) =>
  new Date(at).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'UTC',
  }) + ' UTC';
// What a step decided, in a few words.
export function stepDecision(output: unknown) {
  const o = output as Record<string, unknown>;
  if (o.condition !== undefined)
    return o.condition ? 'Condition matched' : 'Condition did not match';
  if (o.exit_reason) return 'Exit: ' + String(o.exit_reason).replaceAll('_', ' ');
  if (o.wait_until) return 'Waiting until ' + sandboxTime(String(o.wait_until));
  return o.port ? 'Continued via ' + String(o.port) : 'Step recorded';
}
