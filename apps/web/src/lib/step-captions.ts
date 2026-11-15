import type { Workflow } from './workflow.ts';
import { durationLabel } from './workflow-presentation.ts';

// The words that sit on top of a step and say when a customer arrives there, so a journey reads
// from top to bottom as a sentence: "Starts when … If purchase arrives … Otherwise, after 1 day …".
export type StepCaption = { text: string; tone: 'start' | 'if' | 'else' | 'variant' };

export function stepCaptions(
  workflow: Pick<Workflow, 'nodes' | 'edges'>,
): Record<string, StepCaption> {
  const nodes = new Map(workflow.nodes.map((node) => [node.id, node]));
  const arriving = new Map<string, StepCaption[]>();
  for (const edge of workflow.edges) {
    const source = nodes.get(edge.source);
    if (!source) continue;
    let caption: StepCaption | undefined;
    if (source.type === 'wait_event')
      caption =
        edge.port === 'timeout'
          ? {
              text: `Otherwise, after ${durationLabel(source.config.timeout_seconds || 0)}`,
              tone: 'else',
            }
          : { text: `If ${source.config.event_name || 'the event'} arrives`, tone: 'if' };
    else if (source.type === 'switch') {
      const branch = source.config.branches?.[Number(edge.port.slice(5)) - 1];
      caption =
        edge.port === 'else'
          ? { text: 'Otherwise', tone: 'else' }
          : { text: `If ${branch?.label || 'this branch matches'}`, tone: 'if' };
    } else if (source.type === 'split') {
      const share =
        edge.port === 'b' ? 100 - (source.config.percent || 0) : source.config.percent || 0;
      caption = {
        text: `Variant ${edge.port === 'b' ? 'B' : 'A'} · ${share}% of customers`,
        tone: 'variant',
      };
    } else if (source.type === 'condition')
      caption =
        edge.port === 'false'
          ? { text: 'Otherwise', tone: 'else' }
          : { text: 'If the condition is met', tone: 'if' };
    arriving.set(edge.target, [...(arriving.get(edge.target) || []), caption!].filter(Boolean));
  }
  const captions: Record<string, StepCaption> = {};
  for (const node of workflow.nodes) {
    if (node.type === 'event_trigger') {
      captions[node.id] = { text: 'Starts when', tone: 'start' };
      continue;
    }
    const incoming = workflow.edges.filter((edge) => edge.target === node.id).length;
    const branches = arriving.get(node.id) || [];
    // A step several different paths lead to has no single condition to state.
    if (branches.length === 1 && incoming === 1) captions[node.id] = branches[0]!;
  }
  return captions;
}
