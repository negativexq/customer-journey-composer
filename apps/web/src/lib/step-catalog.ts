import type { Config, Kind } from './workflow.ts';

// How the builder names, groups and explains each step type.

// The short name shown on a connection.
export const branchLabel = (port: string) =>
  (
    ({
      true: 'Yes',
      false: 'No',
      matched: 'Matched',
      timeout: 'Timeout',
      a: 'A',
      b: 'B',
    }) as Record<string, string>
  )[port] || (port === 'else' ? 'Otherwise' : port.startsWith('case_') ? port.slice(5) : '');

export const iconFor: Record<string, string> = {
  event_trigger: 'zap',
  wait: 'clock',
  wait_event: 'events',
  condition: 'branch',
  switch: 'branch',
  split: 'split',
  email: 'email',
  sms: 'message',
  webhook: 'webhook',
  update_attribute: 'user',
  exit: 'exit',
};

// Headings for the step-type menu in the insert dialog.
export const stepGroup: Record<string, string> = {
  wait: 'Timing',
  wait_event: 'Timing',
  condition: 'Branching',
  switch: 'Branching',
  split: 'Branching',
  email: 'Actions',
  sms: 'Actions',
  webhook: 'Actions',
  update_attribute: 'Actions',
  exit: 'End',
};

// The heading over a step's own settings.
export const groupTitle: Record<string, string> = {
  event_trigger: 'Entry event',
  wait: 'Timing',
  wait_event: 'Event to wait for',
  condition: 'Rule',
  switch: 'Branches',
  split: 'Experiment',
  email: 'Email content',
  sms: 'Message',
  webhook: 'Request',
  update_attribute: 'Profile change',
  exit: 'Outcome',
};

// One sentence on what a step does, shown at the top of its settings.
export const explanations: Record<string, string> = {
  event_trigger:
    'A customer enters each time the entry event arrives for them, or, with an audience, when they match it at the scheduled time.',
  wait: 'Holds the customer here for a set time, then moves on.',
  wait_event:
    'Watches for an event. The customer takes Matched if it arrives in time and Timeout if it does not.',
  condition: 'Checks the customer against a rule and sends them down Yes or No.',
  switch:
    'Checks the branches from top to bottom and sends the customer down the first one that matches, or Otherwise if none does.',
  split:
    'Sends a fixed share of customers to variant A and the rest to B. A customer always gets the same variant here.',
  email: 'Sends an email, if the customer allows email and the sending limit has room.',
  sms: 'Sends a text message, if the customer allows SMS and the sending limit has room.',
  webhook:
    'Posts the chosen customer and event fields to another system as JSON. Only hosts the server allows are called; others are recorded but not sent.',
  update_attribute: 'Changes a value on the customer profile for later steps and journeys.',
  exit: 'Ends the journey for this customer and records why.',
};

// A few words on each step type, for menus.
export const descriptions: Record<string, string> = {
  event_trigger: 'Start from a customer event',
  wait: 'Wait before the next step',
  wait_event: 'Listen for an event',
  condition: 'Split by a condition',
  switch: 'Split into several paths',
  split: 'Test two variants',
  email: 'Send an email message',
  sms: 'Send a text message',
  webhook: 'Notify another system',
  update_attribute: 'Change a customer attribute',
  exit: 'Complete this journey',
};

// A step and a connection as the canvas holds them.
export type CanvasData = { kind: Kind; label: string; config: Config };
export type CanvasNode = {
  id: string;
  type: string;
  position: { x: number; y: number };
  data: CanvasData;
  selected?: boolean;
  class?: string;
};
export type CanvasEdge = {
  type?: 'insertable';
  data?: Record<string, unknown>;
  id: string;
  source: string;
  target: string;
  sourceHandle?: string | null;
  targetHandle?: string;
  label?: string;
};
