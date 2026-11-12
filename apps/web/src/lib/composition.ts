import type { Workflow } from './workflow';
import type { Journey } from './api';
export type Proposal = {
  action?: string;
  diff?: {
    added_nodes: string[];
    removed_nodes: string[];
    updated_nodes: string[];
    changed_conditions: string[];
    changed_messages: string[];
  };
  schema_version: string;
  name: string;
  description: string;
  workflow: Workflow;
  explanation: string;
  warnings: string[];
  generation?: {
    attempts: unknown[];
    usage: { input_tokens: number; output_tokens: number } | null;
  };
};
export type CompositionState = {
  step: 'details' | 'method' | 'brief' | 'review' | 'builder';
  method?: 'ai' | 'scratch';
  provider?: 'openai' | 'claude';
  acceptance_contract?: string;
  brief?: string;
  changes?: string;
  editing?: boolean;
  generating?: boolean;
  proposal?: Proposal;
};
export type CompositionRecord = { journey: Journey; state: CompositionState };
