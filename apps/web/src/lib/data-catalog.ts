import type { Workflow } from './workflow';
export type DataEntry = {
  key: string;
  kind: 'event' | 'customer_field' | 'event_field';
  event_name?: string;
  path?: string;
  label: string;
  description?: string;
  data_type: 'event' | 'string' | 'number' | 'boolean' | 'mixed';
  aliases: string[];
  enabled: boolean;
  source?: string;
};
export type DataMapping = {
  reference: { key: string; kind: string; event_name?: string; path?: string };
  label: string;
  status: 'matched' | 'confirmed' | 'missing' | 'ambiguous' | 'mixed' | 'type_mismatch';
  candidates: DataEntry[];
};
export type GroundingReport = { version: string; ready: boolean; mappings: DataMapping[] };
export type DataResolution = { workflow: Workflow; grounding: GroundingReport };
