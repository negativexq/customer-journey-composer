import type { DeliveryEngagement } from './engagement.ts';

// One customer's run through a published journey, as the executions API returns it.
export type Execution = {
  id: string;
  version_id: string;
  journey_id: string;
  entry_event_id: string;
  customer_id: string;
  state: string;
  current_node_id: string;
  started_at: string;
  exit_reason?: string;
  steps: {
    node_id: string;
    state: string;
    output: { port?: string; condition?: boolean; exit_reason?: string; error?: string };
    started_at: string;
    completed_at?: string;
  }[];
  jobs: {
    id: string;
    node_id: string;
    kind: string;
    state: string;
    due_at: string;
    attempts: number;
  }[];
  waiters: { node_id: string; event_name: string; state: string; deadline: string }[];
  deliveries: {
    id: string;
    node_id: string;
    channel: string;
    status: string;
    decision: unknown;
    attempts: number;
    last_error?: string;
  }[];
};

// One message of an execution with its policy decision and provider attempts.
export type DeliveryDetail = {
  id: string;
  provider: string;
  execution_id?: string;
  status: string;
  decision: unknown;
  attempt_history: unknown[];
  engagement?: DeliveryEngagement[];
  last_error?: string;
};

// Monitor times are shown in UTC, the zone entry cohorts are counted in.
export const utcTime = (at: string) =>
  new Date(at).toLocaleString('en-US', { timeZone: 'UTC' }) + ' UTC';

// What a finished or running step decided, in a sentence.
export function decisionText(output: Execution['steps'][number]['output']) {
  if (output.error) return output.error;
  if (output.condition !== undefined)
    return output.condition
      ? 'Condition matched · Yes branch'
      : 'Condition did not match · No branch';
  if (output.exit_reason) return 'Exit: ' + output.exit_reason.replaceAll('_', ' ');
  return output.port ? 'Continued via ' + output.port : 'Waiting for this step to finish';
}
