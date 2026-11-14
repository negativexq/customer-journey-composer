import type { Kind, Step, Workflow } from './workflow';

// Turns one execution into the route a person actually took, in words a marketer can read.
export type ExecutionStep = {
  id: string;
  node_id: string;
  node_type: string;
  state: string;
  started_at: string;
  completed_at?: string | null;
  output: { port?: string };
};
export type ExecutionDelivery = {
  id: string;
  node_id: string;
  channel: string;
  status: string;
  decision?: { reason?: string } | null;
  last_error?: string | null;
};
export type ExecutionWaiter = {
  node_id: string;
  event_name: string;
  deadline?: string | null;
  state: string;
};
export type ExecutionDetail = {
  id: string;
  journey_id: string;
  version_id: string;
  customer_id: string;
  entry_event_id?: string;
  state: string;
  exit_reason?: string | null;
  current_node_id: string;
  started_at: string;
  completed_at?: string | null;
  steps: ExecutionStep[];
  deliveries: ExecutionDelivery[];
  waiters: ExecutionWaiter[];
};
export type PathTone = 'done' | 'current' | 'failed' | 'blocked';
export type PathStep = {
  key: string;
  nodeID: string;
  kind: Kind;
  label: string;
  tone: PathTone;
  outcome: string;
  detail: string;
  at: string;
  // Time spent in this step, when it is long enough to matter.
  took: string;
  branch: string;
  deliveryID?: string;
};
export type JourneyPath = {
  steps: PathStep[];
  notReached: Pick<Step, 'id' | 'label' | 'type'>[];
  headline: string;
  duration: string;
};

export function spanLabel(from?: string | null, to?: string | null) {
  if (!from || !to) return '';
  const seconds = Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000);
  if (!Number.isFinite(seconds) || seconds < 1) return '';
  const units: [number, string][] = [
    [86400, 'd'],
    [3600, 'h'],
    [60, 'm'],
    [1, 's'],
  ];
  const parts: string[] = [];
  let rest = seconds;
  for (const [size, unit] of units) {
    if (rest >= size && parts.length < 2) {
      parts.push(Math.floor(rest / size) + unit);
      rest %= size;
    }
  }
  return parts.join(' ');
}

const words = (value: string) => value.replaceAll('_', ' ').toLowerCase();
const channelName = (kind: string) => (kind === 'sms' ? 'SMS' : 'Email');
const blockedReasons: Record<string, string> = {
  consent_denied: 'no permission for this channel',
  frequency_cap: 'sending limit reached',
  missing_contact: 'contact details missing',
};

function describe(
  node: Step | undefined,
  step: ExecutionStep,
  execution: ExecutionDetail,
): Pick<PathStep, 'tone' | 'outcome' | 'branch' | 'deliveryID'> {
  const port = step.output?.port || '';
  const kind = node?.type || (step.node_type as Kind);
  const active = ['RUNNING', 'WAITING'].includes(execution.state);
  if (step.state === 'FAILED') return { tone: 'failed', outcome: 'This step failed', branch: '' };
  if (!step.completed_at && active && step.node_id === execution.current_node_id) {
    const waiter = execution.waiters.find(
      (item) => item.node_id === step.node_id && item.state === 'WAITING',
    );
    return {
      tone: 'current',
      outcome: waiter ? `Waiting for ${waiter.event_name}` : 'Here now',
      branch: '',
    };
  }
  switch (kind) {
    case 'event_trigger':
      return {
        tone: 'done',
        outcome: `Entered when ${node?.config.event_name || 'the event'} arrived`,
        branch: '',
      };
    case 'wait':
      return { tone: 'done', outcome: 'Wait finished', branch: '' };
    case 'wait_event': {
      const event = node?.config.event_name || 'the event';
      return port === 'timeout'
        ? { tone: 'done', outcome: `${event} did not arrive in time`, branch: 'Timeout' }
        : { tone: 'done', outcome: `${event} arrived`, branch: 'Matched' };
    }
    case 'condition':
      return port === 'true'
        ? { tone: 'done', outcome: 'Condition was met', branch: 'Yes' }
        : { tone: 'done', outcome: 'Condition was not met', branch: 'No' };
    case 'email':
    case 'sms': {
      const delivery = execution.deliveries.find((item) => item.node_id === step.node_id);
      const name = channelName(kind);
      if (!delivery) return { tone: 'done', outcome: `${name} step passed`, branch: '' };
      const deliveryID = delivery.id;
      const status = delivery.status.toUpperCase();
      if (['ACCEPTED', 'SIMULATED', 'TEST_ACCEPTED'].includes(status))
        return {
          tone: 'done',
          outcome:
            status === 'SIMULATED' ? `${name} simulated` : `${name} accepted by the provider`,
          branch: '',
          deliveryID,
        };
      if (['BLOCKED', 'SKIPPED', 'SUPPRESSED'].includes(status)) {
        const reason = delivery.decision?.reason || '';
        return {
          tone: 'blocked',
          outcome: `${name} not sent: ${blockedReasons[reason] || words(reason || status)}`,
          branch: '',
          deliveryID,
        };
      }
      if (['FAILED', 'REJECTED'].includes(status))
        return {
          tone: 'failed',
          outcome: `${name} failed at the provider`,
          branch: '',
          deliveryID,
        };
      return { tone: 'done', outcome: `${name} ${words(status)}`, branch: '', deliveryID };
    }
    case 'update_attribute':
      return { tone: 'done', outcome: 'Profile updated', branch: '' };
    case 'exit':
      return {
        tone: 'done',
        outcome: `Left the journey${node?.config.reason ? ': ' + words(String(node.config.reason)) : ''}`,
        branch: '',
      };
    default:
      return { tone: 'done', outcome: 'Step finished', branch: '' };
  }
}

export function journeyPath(
  workflow: Workflow,
  execution: ExecutionDetail,
  summarize: (step: Pick<Step, 'type' | 'config'>) => string,
): JourneyPath {
  const nodes = new Map(workflow.nodes.map((node) => [node.id, node]));
  const steps = execution.steps.map((step): PathStep => {
    const node = nodes.get(step.node_id);
    return {
      key: step.id,
      nodeID: step.node_id,
      kind: node?.type || (step.node_type as Kind),
      label: node?.label || step.node_id,
      detail: node ? summarize(node) : '',
      at: step.started_at,
      took: spanLabel(step.started_at, step.completed_at),
      ...describe(node, step, execution),
    };
  });
  const visited = new Set(execution.steps.map((step) => step.node_id));
  const notReached = workflow.nodes
    .filter((node) => !visited.has(node.id))
    .map(({ id, label, type }) => ({ id, label, type }));
  const current = nodes.get(execution.current_node_id)?.label || execution.current_node_id;
  const reason = execution.exit_reason ? words(execution.exit_reason) : '';
  const headline =
    (
      {
        RUNNING: `In progress at ${current}`,
        WAITING: `Waiting at ${current}`,
        COMPLETED: 'Finished the journey',
        EXITED: `Left early${reason ? ': ' + reason : ''}`,
        FAILED: `Stopped by a failure at ${current}`,
        CANCELLED: 'Cancelled by an operator',
      } as Record<string, string>
    )[execution.state] || words(execution.state);
  return {
    steps,
    notReached,
    headline,
    duration: spanLabel(execution.started_at, execution.completed_at),
  };
}

export type ActivityItem = {
  key: string;
  kind: 'event' | 'message' | 'execution';
  id: string;
  at: string;
  title: string;
  detail: string;
  status: string;
};
// One chronological feed from the three histories the API returns separately. Newest first.
export function activityFeed(
  events: { event_id: string; event_name: string; occurred_at: string; status?: string }[],
  deliveries: {
    id: string;
    channel: string;
    mode?: string;
    status: string;
    created_at?: string;
    accepted_at?: string | null;
  }[],
  executions: { id: string; state: string; started_at: string }[],
  journeyName: (execution: { id: string }) => string,
): ActivityItem[] {
  const items: ActivityItem[] = [
    ...events.map((event) => ({
      key: 'event:' + event.event_id,
      kind: 'event' as const,
      id: event.event_id,
      at: event.occurred_at,
      title: event.event_name,
      detail: 'Event received',
      status: event.status || '',
    })),
    ...deliveries.map((delivery) => ({
      key: 'message:' + delivery.id,
      kind: 'message' as const,
      id: delivery.id,
      at: delivery.created_at || delivery.accepted_at || '',
      title: `${channelName(delivery.channel)} message`,
      detail: delivery.mode && delivery.mode !== 'LIVE' ? words(delivery.mode) : 'Live send',
      status: delivery.status,
    })),
    ...executions.map((execution) => ({
      key: 'execution:' + execution.id,
      kind: 'execution' as const,
      id: execution.id,
      at: execution.started_at,
      title: `Entered ${journeyName(execution)}`,
      detail: 'Journey entry',
      status: execution.state,
    })),
  ];
  return items.sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime());
}

export function displayName(customer: {
  customer_id: string;
  email?: string;
  attributes?: Record<string, unknown> | null;
}) {
  const attributes = customer.attributes || {};
  const full = [attributes.first_name, attributes.last_name]
    .filter((part): part is string => typeof part === 'string' && !!part.trim())
    .join(' ');
  return full || customer.email || customer.customer_id;
}
export function initials(name: string) {
  const parts = name
    .split('@')[0]!
    .split(/[\s._-]+/)
    .filter(Boolean);
  return (
    parts
      .slice(0, 2)
      .map((part) => part[0]!.toUpperCase())
      .join('') || '?'
  );
}
