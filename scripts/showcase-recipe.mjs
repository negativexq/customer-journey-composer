export const showcaseName = 'Abandoned Cart Recovery';
export const showcaseDescription =
  'Recover abandoned carts with a personal email and a timely 10% offer.';
export const showcaseTag = 'Cart recovery';
const node = (id, type, label, x, y, config) => ({ id, type, label, position: { x, y }, config });
const edge = (source, target, port) => ({ id: source + '_' + port, source, target, port });
export function showcaseRecipe({ accelerated = false } = {}) {
  const firstWait = accelerated ? 5 : 3600;
  const purchaseWait = accelerated ? 30 : 86400;
  const workflow = {
    schema_version: 1,
    reentry: { mode: 'cooldown', cooldown_seconds: 604800 },
    global_exit: {
      event_name: 'purchase_completed',
      reason: 'converted',
      correlation: {
        entry_path: 'event.properties.cart_id',
        event_path: 'event.properties.cart_id',
      },
    },
    nodes: [
      node('cart', 'event_trigger', 'Cart updated', 360, 0, {
        event_name: 'cart_updated',
        filters: {
          logic: 'and',
          rules: [
            { path: 'event.properties.amount', operator: 'greater_than_or_equal', value: 50 },
            { path: 'event.properties.currency', operator: 'equals', value: 'EUR' },
          ],
        },
      }),
      node('reminder_wait', 'wait', 'Wait before reminder', 360, 190, {
        duration_seconds: firstWait,
        timezone: 'UTC',
      }),
      node('email', 'email', 'Your cart is waiting', 360, 380, {
        subject: '{{ customer.attributes.first_name }}, your cart is waiting',
        preheader: 'Your Northstar favourites, right where you left them.',
        html: '<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;background-color:#f3efe8;font-family:Helvetica, Arial, sans-serif"><tr><td style="padding:32px 16px"><table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;max-width:560px;margin:0 auto;background-color:#ffffff;border-radius:16px"><tr><td style="padding:24px 32px;border-bottom:1px solid #ece6dc"><p style="margin:0;font-size:15px;font-weight:700;letter-spacing:4px;color:#1f2430">NORTHSTAR</p></td></tr><tr><td style="padding:36px 32px 8px"><h1 style="margin:0 0 16px;font-size:30px;line-height:36px;font-weight:700;letter-spacing:-0.5px;color:#1f2430">Made for your everyday.</h1><p style="margin:0 0 12px;font-size:16px;line-height:24px;color:#1f2430">Hi {{ customer.attributes.first_name }},</p><p style="margin:0;font-size:16px;line-height:24px;color:#5b6170">We saved your favourites. Pick up where you left off whenever you are ready.</p></td></tr><tr><td style="padding:24px 32px"><table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;border:1px solid #ece6dc;border-radius:12px"><tr><td style="width:64px;padding:16px"><table role="presentation" cellpadding="0" cellspacing="0" style="width:64px;height:64px;background-color:#e9dfcf;border-radius:10px"><tr><td style="text-align:center;vertical-align:middle;font-size:11px;font-weight:700;letter-spacing:2px;color:#8a7458">TOTE</td></tr></table></td><td style="padding:16px 16px 16px 4px;vertical-align:middle"><p style="margin:0 0 4px;font-size:16px;font-weight:700;color:#1f2430">Everyday Carry Tote</p><p style="margin:0 0 8px;font-size:14px;color:#5b6170">Natural canvas · 1 item</p><p style="margin:0;font-size:18px;font-weight:700;color:#1f2430">{{ event.properties.amount_display }}</p></td></tr></table></td></tr><tr><td style="padding:8px 32px 36px;text-align:center"><a href="https://northstar.example.invalid/cart" style="display:inline-block;padding:15px 32px;background-color:#1f2430;border-radius:10px;font-size:16px;font-weight:700;color:#ffffff;text-decoration:none">Return to your cart →</a></td></tr><tr><td style="padding:20px 32px;border-top:1px solid #ece6dc;text-align:center"><p style="margin:0;font-size:13px;line-height:20px;color:#8a8f9c">A little reminder from Northstar.</p></td></tr></table></td></tr></table>',
        text: 'Hi {{ customer.attributes.first_name }}, your Northstar cart of {{ event.properties.amount_display }} is waiting. Return to your cart: https://northstar.example.invalid/cart',
        fallbacks: {
          'customer.attributes.first_name': 'there',
          'event.properties.amount_display': 'your saved items',
        },
      }),
      node('purchase_wait', 'wait_event', 'Wait for purchase', 360, 570, {
        event_name: 'purchase_completed',
        timeout_seconds: purchaseWait,
      }),
      node('sms', 'sms', 'A little extra incentive', 580, 790, {
        text: 'Hi {{ customer.attributes.first_name }}! Your {{ event.properties.amount_display }} Northstar cart is waiting. Take 10% off with COMEBACK10. https://northstar.example.invalid/cart',
        fallbacks: {
          'customer.attributes.first_name': 'there',
          'event.properties.amount_display': 'saved',
        },
      }),
      node('completed', 'exit', 'Recovery complete', 580, 990, { reason: 'completed' }),
      node('converted', 'exit', 'Purchase completed', 80, 790, { reason: 'converted' }),
    ],
    edges: [
      edge('cart', 'reminder_wait', 'started'),
      edge('reminder_wait', 'email', 'completed'),
      edge('email', 'purchase_wait', 'next'),
      edge('purchase_wait', 'converted', 'matched'),
      edge('purchase_wait', 'sms', 'timeout'),
      edge('sms', 'completed', 'next'),
    ],
  };
  const start = '2026-11-13T09:00:00Z';
  const customer = {
    customer_id: 'alex-morgan',
    email: 'alex.morgan@example.invalid',
    phone: '+12025550100',
    attributes: { first_name: 'Alex', last_name: 'Morgan' },
    consents: { email: true, sms: true },
  };
  const properties = {
    cart_id: 'CART-ALEX-001',
    amount: 89.9,
    amount_display: '€89.90',
    currency: 'EUR',
  };
  const scenario = (name, email, sms, outcome) => ({
    name,
    start_time: start,
    customer: structuredClone(customer),
    entry_event: {
      event_id: 'cart-alex',
      event_name: 'cart_updated',
      properties: structuredClone(properties),
    },
    frequency_limit: 3,
    frequency_window_seconds: 86400,
    assertions: [
      { type: 'enrollment_count', expected: 1 },
      { type: 'message_count', channel: 'email', expected: email },
      { type: 'message_count', channel: 'sms', expected: sms },
      { type: 'exit_reason', expected: outcome },
      { type: 'no_duplicate_delivery', expected: true },
    ],
  });
  const full = scenario('Email and SMS · no purchase', 1, 1, 'completed');
  const early = scenario('Purchase before the reminder', 0, 0, 'converted');
  early.events = [
    {
      after_seconds: accelerated ? 2 : 1800,
      event: {
        event_id: 'purchase-early',
        event_name: 'purchase_completed',
        properties: { cart_id: properties.cart_id },
      },
    },
  ];
  const afterEmail = scenario('Purchase after the email', 1, 0, 'converted');
  afterEmail.events = [
    {
      after_seconds: accelerated ? 10 : 7200,
      event: {
        event_id: 'purchase-after-email',
        event_name: 'purchase_completed',
        properties: { cart_id: properties.cart_id },
      },
    },
  ];
  const noSMS = scenario('Email consent only', 1, 0, 'completed');
  noSMS.customer.consents.sms = false;
  const unrelated = scenario('Another cart is purchased', 1, 1, 'completed');
  unrelated.events = [
    {
      after_seconds: accelerated ? 10 : 7200,
      event: {
        event_id: 'different-cart',
        event_name: 'purchase_completed',
        properties: { cart_id: 'ANOTHER-CART' },
      },
    },
  ];
  const contract = {
    version: 2,
    confirmed: true,
    case_name: 'No purchase during recovery',
    entry_event: 'cart_updated',
    conditions: [
      { path: 'event.properties.amount', operator: 'greater_than_or_equal', value: 50 },
      { path: 'event.properties.currency', operator: 'equals', value: 'EUR' },
    ],
    inputs: {
      'customer.attributes.first_name': 'Alex',
      ...Object.fromEntries(
        Object.entries(properties).map(([k, v]) => ['event.properties.' + k, v]),
      ),
    },
    consents: { email: true, sms: true },
    email_count: 1,
    sms_count: 1,
    outcome: 'completed',
    duration_seconds: firstWait + purchaseWait,
    alternatives: [
      {
        name: 'Purchase before the first reminder',
        email_count: 0,
        sms_count: 0,
        outcome: 'converted',
        duration_seconds: accelerated ? 2 : 1800,
        events: [
          {
            event_name: 'purchase_completed',
            after_seconds: accelerated ? 2 : 1800,
            properties: { cart_id: properties.cart_id },
            entry_binding: {
              entry_path: 'event.properties.cart_id',
              event_path: 'event.properties.cart_id',
              operator: 'equals',
            },
          },
        ],
      },
    ],
  };
  workflow.behavior_checks = contract;
  const catalog = [
    ...[
      ['cart_updated', 'Cart updated'],
      ['purchase_completed', 'Purchase completed'],
    ].map(([event_name, label]) => ({
      kind: 'event',
      event_name,
      label,
      data_type: 'event',
      aliases: [],
      enabled: true,
    })),
    ...[
      ['first_name', 'First name'],
      ['last_name', 'Last name'],
    ].map(([key, label]) => ({
      kind: 'customer_field',
      path: 'customer.attributes.' + key,
      label,
      data_type: 'string',
      aliases: [],
      enabled: true,
    })),
    ...[
      ['cart_updated', 'cart_id', 'Cart ID', 'string'],
      ['purchase_completed', 'cart_id', 'Cart ID', 'string'],
      ['cart_updated', 'amount', 'Cart total', 'number'],
      ['cart_updated', 'currency', 'Currency', 'string'],
      ['cart_updated', 'amount_display', 'Formatted cart total', 'string'],
    ].map(([event_name, key, label, data_type]) => ({
      kind: 'event_field',
      event_name,
      path: 'event.properties.' + key,
      label,
      data_type,
      aliases: [],
      enabled: true,
    })),
  ];
  return { workflow, contract, catalog, scenarios: [full, early, afterEmail, noSMS, unrelated] };
}
