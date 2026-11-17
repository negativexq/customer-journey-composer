import { diffWorkflows, type StepChange } from './version-diff.ts';
import { stepSummary } from './workflow-presentation.ts';
import { labels, messageKinds, type Workflow } from './workflow.ts';

const messages = new Set<string>(messageKinds);

/**
 * What an AI proposal changes in steps that already exist, as "before → after"
 * lines. The model can change more than it was asked, for example a wait it
 * was told to keep, so changes to timing, conditions and other step settings
 * are listed apart: a person confirms them before applying.
 */
export function proposalChanges(before: Workflow, after: Workflow) {
  const changed = diffWorkflows(before, after, stepSummary, (step) => labels[step.type]).changed;
  const behavior: StepChange[] = changed.filter(
    (change) =>
      !messages.has(change.step.type) &&
      change.changes.some((line) => !line.startsWith('Renamed from')),
  );
  return { changed, behavior };
}
