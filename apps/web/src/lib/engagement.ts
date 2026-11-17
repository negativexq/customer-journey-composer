import type { Channel } from './workflow.ts';
// What a customer can do with a sent message. The delivery service turns each report into a
// customer event of this name, so a "Wait for event" step can wait for one message's engagement.
export type EngagementEvent = {
  value: string;
  label: string;
  description: string;
  channels: Channel[];
  // Completes "If …" with the message step's name.
  phrase: (message: string) => string;
};

export const engagementEvents: EngagementEvent[] = [
  {
    value: 'message_clicked',
    label: 'Clicks a link',
    description: 'Any tracked link in the message, or a tap on a push',
    channels: ['email', 'sms', 'push'],
    phrase: (m) => `they click a link in ${m}`,
  },
  {
    value: 'message_opened',
    label: 'Opens the email',
    description: 'Counted when the mail client loads its images',
    channels: ['email'],
    phrase: (m) => `they open ${m}`,
  },
  {
    value: 'message_delivered',
    label: 'Is delivered',
    description: 'The provider reports it reached the inbox, phone or device',
    channels: ['email', 'sms', 'push'],
    phrase: (m) => `${m} is delivered`,
  },
  {
    value: 'message_bounced',
    label: 'Bounces',
    description: 'The provider could not deliver it',
    channels: ['email', 'sms', 'push'],
    phrase: (m) => `${m} bounces`,
  },
  {
    value: 'message_unsubscribed',
    label: 'Unsubscribes',
    description: 'They use the email’s unsubscribe link',
    channels: ['email'],
    phrase: (m) => `they unsubscribe from ${m}`,
  },
];

export const engagementEvent = (name?: string) => engagementEvents.find((e) => e.value === name);

// A report kept for one delivery: the first of each kind.
export type DeliveryEngagement = {
  kind: string;
  occurred_at: string;
  source: string;
  detail?: { url?: string; reason?: string };
};
