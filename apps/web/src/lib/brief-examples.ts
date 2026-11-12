export const briefExamples = [
  {
    title: 'Trial conversion',
    description: 'Usage-based guidance · subscription goal',
    brief:
      'Create a SaaS trial conversion journey. Enter on trial_started once per customer. Exit converted immediately on subscription_started at any active stage. If customer.attributes.is_subscriber is true at entry, exit converted without messages. Otherwise send a welcome email if email consent exists. Wait 3 days for subscription_started, then check is_subscriber again. If still false and customer.attributes.login_count < 3 and email consent exists, send a getting-started guide. Wait 2 more days, check is_subscriber, and send an upgrade reminder SMS only if not subscribed and SMS consent exists. Wait 2 more days, check is_subscriber, then exit converted if true or completed otherwise. Use customer.attributes.first_name and product_name dynamically with fallbacks. Respect customer.consents.email/sms and runtime frequency caps. Use https://example.com/upgrade as a placeholder and warn before publishing.',
  },
  {
    title: 'Abandoned cart',
    description: '€50+ carts · email and SMS recovery',
    brief: '',
    contract: 'cart-recovery-eur.v1',
  },
  {
    title: 'Welcome & activation',
    description: 'Welcome email · activation reminder',
    brief:
      'Create an onboarding journey triggered by user_registered, once per customer. Exit converted globally on account_activated. Check customer.attributes.activated at entry and exit converted without messages if true. Otherwise send a welcome email only with customer.consents.email. Wait 48 hours for account_activated, check activated again, and if still false send an activation reminder only with customer.consents.sms. Wait another 24 hours, check activated and exit converted if true or completed otherwise. If neither channel has consent, exit condition_not_met immediately. Personalize first_name and product_name from customer.attributes, with fallbacks. Use https://example.com/activate as a placeholder and warn before publishing.',
  },
  {
    title: 'Win-back',
    description: 'Inactive customers · one gentle follow-up',
    brief:
      'Create a win-back journey triggered by customer_inactive. Permit reentry only after a 30-day cooldown. If neither email nor SMS has consent, exit condition_not_met immediately. Send a personalized re-engagement email only with customer.consents.email. Wait 7 days for session_started. session_started must exit converted globally at every active stage. At timeout, check customer.attributes.reactivated; if true exit converted. Otherwise send one reminder SMS only with customer.consents.sms. Wait 3 more days and check reactivated again; true means converted, false means completed. Use customer.attributes.first_name and product_name dynamically with fallbacks. Respect runtime frequency caps. Use https://example.com/return as a placeholder; warn that the URL must be replaced.',
  },
];
