export const briefExamples: {
  title: string;
  description: string;
  brief: string;
  contract?: string;
  expectations?: { emails: number; sms: number; hours: number };
}[] = [
  {
    title: 'Trial conversion',
    expectations: { emails: 2, sms: 1, hours: 168 },
    description: 'Usage-based guidance · subscription goal',
    brief:
      'Help new trial customers become subscribers. Let each customer enter once. If they are already subscribed, finish successfully without sending anything. Otherwise send a welcome email when permitted. After three days, check their subscription again. If they have not subscribed and have logged in fewer than three times, send a getting-started guide by email when permitted. Two days later, check again and send an upgrade reminder by SMS if they still have not subscribed and allow SMS. Wait two more days and finish successfully if they subscribed, or without conversion otherwise. Stop all remaining steps immediately when they subscribe. Use the customer’s first name and product name in messages, with sensible fallback wording. Respect channel preferences and sending limits. Use https://example.com/upgrade as a placeholder that must be replaced before launch.',
  },
  {
    title: 'Abandoned cart',
    description: '€50+ carts · email and SMS recovery',
    brief: '',
    contract: 'cart-recovery-eur.v1',
  },
  {
    title: 'Welcome & activation',
    expectations: { emails: 1, sms: 1, hours: 72 },
    description: 'Welcome email · activation reminder',
    brief:
      'Welcome newly registered customers and help them activate their accounts. Let each customer enter once. If their account is already active, finish successfully without sending messages. If neither email nor SMS is permitted, finish immediately without messages. Otherwise send a personalized welcome email when email is permitted. Wait up to two days for activation, then check the current account status. If they are still inactive, send an activation reminder only when SMS is permitted. Wait another day, check again and finish successfully if active, or without conversion otherwise. End the journey immediately whenever the customer activates their account. Include their first name and product name, using sensible fallback wording when either is missing. Use https://example.com/activate as a placeholder and flag it for replacement before launch.',
  },
  {
    title: 'Win-back',
    expectations: { emails: 1, sms: 1, hours: 240 },
    description: 'Inactive customers · one gentle follow-up',
    brief:
      'Bring inactive customers back to the product. Customers may enter again only after thirty days. If no communication channel is permitted, finish immediately without messages. Send a personalized re-engagement email when email is permitted, then wait seven days for the customer to return. If they return at any point, finish successfully and stop remaining messages. After the wait, check whether they have returned; if not, send one SMS reminder when permitted. Wait three more days, check again and finish successfully if they returned, or without conversion otherwise. Include the customer’s first name and product name, with sensible fallback wording. Respect sending limits and use https://example.com/return as a placeholder that must be replaced before launch.',
  },
];
