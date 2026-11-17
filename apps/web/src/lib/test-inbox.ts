import type { DeliveryEngagement } from './engagement.ts';

// A message in the Test Inbox list.
export type TestInboxRow = {
  id: string;
  test_run_id?: string;
  delivery_id?: string;
  journey_id?: string;
  journey_name?: string;
  customer_id?: string;
  node_label: string;
  source_node?: string;
  channel: string;
  mode: string;
  status: string;
  simulated_at: string;
  created_at: string;
  version?: number;
};
// One test message with what it rendered, why it was allowed and what the provider did.
export type TestMessage = TestInboxRow & {
  message: {
    subject?: string;
    preheader?: string;
    html?: string;
    text?: string;
    to?: string;
    personalization_values?: Record<string, string>;
    template_id?: string;
    encoding?: string;
    segments?: number;
  };
  decision: unknown;
  attempt_history: unknown[];
  engagement?: DeliveryEngagement[];
  provider_response?: string;
  provider_status?: string;
  provider?: string;
  execution_trace: unknown[];
  trace_id: string;
  workflow_hash?: string;
};
