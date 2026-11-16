// How a sandbox run is named in lists: by what was tested rather than by its ID.
export type TestRunSummary = {
  id: string;
  result: string;
  execution_state: string;
  created_at?: string;
  journey_name?: string;
  scenario_name?: string | null;
  version?: number | null;
  draft_revision?: number | null;
  mode?: string;
};

export function runTitle(run: TestRunSummary): string {
  return (
    run.scenario_name?.trim() || (run.mode === 'SINGLE_NODE' ? 'Single-step test' : 'Test run')
  );
}

export function runContext(run: TestRunSummary): string {
  const source = run.version
    ? `Version ${run.version}`
    : run.draft_revision
      ? `Draft edit ${run.draft_revision}`
      : 'Draft';
  return [run.journey_name, source, run.mode === 'SINGLE_NODE' ? 'One step' : '']
    .filter(Boolean)
    .join(' · ');
}
