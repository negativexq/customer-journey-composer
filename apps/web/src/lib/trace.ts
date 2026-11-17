// Turns a sandbox execution trace (flat log lines) into steps a person can read:
// each step a customer entered, with what happened there folded underneath.
// Keep this free of runtime imports so Node tests can load it.

export type TraceEntry = {
  at: string;
  kind: string;
  node_id?: string;
  message: string;
  execution: number;
};
export type TraceTone = 'green' | 'amber' | 'red' | 'blue' | 'neutral';
export type TraceDetail = { text: string; tone?: TraceTone; status?: string };
export type TraceItem =
  | {
      type: 'step';
      nodeID: string;
      label: string;
      at: string;
      execution: number;
      details: TraceDetail[];
    }
  | {
      type: 'note';
      kind: string;
      title: string;
      // For events: a small caption ("Event received") over the event name.
      caption?: string;
      subject?: string;
      at: string;
      execution: number;
      tone: TraceTone;
      status?: string;
    };

const statusTone: Record<string, TraceTone> = {
  ACCEPTED: 'green',
  TEST_ACCEPTED: 'green',
  SIMULATED: 'blue',
  LATE: 'amber',
  DUPLICATE: 'amber',
  BLOCKED: 'amber',
  SKIPPED: 'amber',
  TEST_PENDING: 'amber',
  REJECTED: 'red',
  FAILED: 'red',
};
const toneOf = (status: string): TraceTone => statusTone[status] || 'neutral';
const label = (status: string) =>
  status.charAt(0) + status.slice(1).toLowerCase().replaceAll('_', ' ');

// "ACCEPTED: cart_updated" → ["ACCEPTED", "cart_updated"].
function split(message: string): [string, string] {
  const m = /^([A-Z_]+): (.*)$/s.exec(message);
  return m ? [m[1], m[2]] : ['', message];
}

/** Short elapsed time from the first trace line: "start", "+45m", "+1h 30m", "+2d". */
export function elapsed(from: string, to: string): string {
  const minutes = Math.round((new Date(to).getTime() - new Date(from).getTime()) / 60000);
  if (!Number.isFinite(minutes) || minutes <= 0) return 'start';
  if (minutes < 60) return `+${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `+${hours}h${minutes % 60 ? ` ${minutes % 60}m` : ''}`;
  const days = Math.floor(hours / 24);
  return `+${days}d${hours % 24 ? ` ${hours % 24}h` : ''}`;
}

function waitText(message: string, at: string, format: (iso: string) => string): string {
  const until = message.replace(/^Waiting until /, '');
  if (Number.isNaN(new Date(until).getTime())) return message;
  const gap = elapsed(at, until);
  return `Waits until ${format(until)}${gap === 'start' ? '' : ` (${gap.slice(1)} later)`}`;
}

function conditionText(message: string): TraceDetail {
  const port = message.replace(/^Condition /, '');
  if (port === 'true') return { text: 'Matched: took the Yes path', tone: 'green' };
  if (port === 'false') return { text: 'Did not match: took the No path', tone: 'neutral' };
  if (port === 'default') return { text: 'No branch matched: took the default path' };
  return { text: `Took the ${port.replaceAll('_', ' ')} path` };
}

function detailFor(entry: TraceEntry, format: (iso: string) => string): TraceDetail | undefined {
  switch (entry.kind) {
    case 'condition':
      return conditionText(entry.message);
    case 'wait':
      return { text: waitText(entry.message, entry.at, format), tone: 'blue' };
    case 'message': {
      const [status] = split(entry.message);
      return { text: 'Message produced', status: label(status), tone: toneOf(status) };
    }
    case 'provider': {
      const status = entry.message.replace(/^Sandbox /, '');
      return { text: 'Sandbox provider answered', status: label(status), tone: toneOf(status) };
    }
    case 'exit':
      return {
        text: `Left the journey: ${entry.message.replace(/^Global exit: /, 'global exit, ').replaceAll('_', ' ')}`,
        tone: 'neutral',
      };
    case 'entry':
      return entry.message === 'Customer enrolled'
        ? { text: 'Customer enrolled', tone: 'green' }
        : { text: entry.message, tone: 'amber' };
    case 'webhook':
      return { text: entry.message.replace(/^Would call /, 'Would call ') };
    case 'error':
      return { text: entry.message, tone: 'red' };
    case 'node':
      return entry.message.startsWith('Entered ') ? undefined : { text: entry.message };
    default:
      return { text: entry.message };
  }
}

function noteFor(entry: TraceEntry): TraceItem {
  const base = {
    type: 'note' as const,
    kind: entry.kind,
    at: entry.at,
    execution: entry.execution,
  };
  if (entry.kind === 'event') {
    const [status, rest] = split(entry.message);
    if (!status) return { ...base, title: entry.message, tone: 'neutral' };
    const title =
      status === 'ACCEPTED'
        ? `Event received: ${rest}`
        : status === 'REJECTED'
          ? `Event rejected: ${rest}`
          : `Event ${label(status).toLowerCase()}`;
    const caption =
      status === 'ACCEPTED'
        ? 'Event received'
        : status === 'REJECTED'
          ? 'Event rejected'
          : `Event ${label(status).toLowerCase()}`;
    return {
      ...base,
      title,
      caption,
      subject: status === 'ACCEPTED' || status === 'REJECTED' ? rest : undefined,
      tone: toneOf(status),
      status: label(status),
    };
  }
  if (entry.kind === 'attributes') return { ...base, title: entry.message, tone: 'blue' };
  if (entry.kind === 'error') return { ...base, title: entry.message, tone: 'red' };
  return { ...base, title: entry.message, tone: 'neutral' };
}

/**
 * Folds the flat trace into steps. "Entered X" opens a step; later lines about the
 * same node attach to the latest visit of that node, even when they arrive after the
 * customer moved on (a provider answer, for example). Lines without a node stay notes.
 */
export function readableTrace(
  trace: TraceEntry[],
  format: (iso: string) => string = (iso) => iso,
): TraceItem[] {
  const items: TraceItem[] = [];
  const latest = new Map<string, Extract<TraceItem, { type: 'step' }>>();
  for (const entry of trace) {
    const key = `${entry.execution}:${entry.node_id}`;
    if (entry.kind === 'node' && entry.message.startsWith('Entered ') && entry.node_id) {
      const step = {
        type: 'step' as const,
        nodeID: entry.node_id,
        label: entry.message.slice('Entered '.length),
        at: entry.at,
        execution: entry.execution,
        details: [] as TraceDetail[],
      };
      items.push(step);
      latest.set(key, step);
      continue;
    }
    const step = entry.node_id ? latest.get(key) : undefined;
    if (!step) {
      // Enrollment is logged before the start step is entered: show it on that step.
      if (entry.kind === 'entry' && entry.node_id) {
        const next = {
          type: 'step' as const,
          nodeID: entry.node_id,
          label: entry.node_id,
          at: entry.at,
          execution: entry.execution,
          details: [] as TraceDetail[],
        };
        const detail = detailFor(entry, format);
        if (detail) next.details.push(detail);
        items.push(next);
        latest.set(key, next);
        continue;
      }
      items.push(noteFor(entry));
      continue;
    }
    const detail = detailFor(entry, format);
    if (detail) step.details.push(detail);
  }
  // The enrollment placeholder takes the real label once the start step is entered.
  return items.filter((item, i) => {
    if (item.type !== 'step' || item.label !== item.nodeID) return true;
    const real = items
      .slice(i + 1)
      .find((o) => o.type === 'step' && o.nodeID === item.nodeID && o.execution === item.execution);
    if (real && real.type === 'step') {
      real.details.unshift(...item.details);
      return false;
    }
    return true;
  });
}
