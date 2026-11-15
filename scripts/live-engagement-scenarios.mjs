// Ten live generations against the local stack, each in a fresh workspace.
// The API holds the provider key; this script never sees it. It never publishes
// and never sends: drafts are checked in Test Lab and read back from PostgreSQL.
//   node scripts/live-engagement-scenarios.mjs [--only id,id] [--out dir]
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';

const base = new URL(process.env.CJC_API_URL || 'http://127.0.0.1:8090');
assert(
  base.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(base.hostname),
  'Live scenarios run against the local stack only.',
);
const arg = (name) => {
  const i = process.argv.indexOf(name);
  return i > 0 ? process.argv[i + 1] : undefined;
};
const only = arg('--only')?.split(',');
const out = arg('--out') || 'docs/validation/live-engagement-2026-11-15';
const START = '2026-11-16T10:00:00Z';
const H = 3600;
const D = 86400;

// ---------- catalog helpers ----------
const ev = (event_name, label) => ({
  kind: 'event',
  event_name,
  label,
  data_type: 'event',
  aliases: [],
  enabled: true,
});
const field = (event_name, name, label, data_type = 'string') => ({
  kind: 'event_field',
  event_name,
  path: 'event.properties.' + name,
  label,
  data_type,
  aliases: [],
  enabled: true,
});
const customer = (name, label, data_type = 'string') => ({
  kind: 'customer_field',
  path: 'customer.attributes.' + name,
  label,
  data_type,
  aliases: [],
  enabled: true,
});
const firstName = customer('first_name', 'Müşterinin adı');
const bind = (prop) => ({
  entry_path: 'event.properties.' + prop,
  event_path: 'event.properties.' + prop,
  operator: 'equals',
});
const LANG =
  ' Mesajlar Türkçe olsun ve müşterinin adını kullansın; ad yoksa "Merhaba" gibi bir yedek kullan.';

// ---------- the ten scenarios ----------
// Each case: inputs/consents/events in; email/sms/outcome/duration expected.
// `contract: true` cases are sent to the model as the human-confirmed contract;
// the others are held out and only run in Test Lab.
const scenarios = [
  {
    id: 'cart_code',
    name: 'Sepet terk – SEPET15 kodu',
    feature: 'discount code in email and SMS, correlated purchase exit, threshold',
    code: 'SEPET15',
    catalog: [
      firstName,
      ev('cart_abandoned', 'Sepet terk edildi'),
      field('cart_abandoned', 'cart_id', 'Sepet numarası'),
      field('cart_abandoned', 'amount', 'Sepet tutarı', 'number'),
      ev('order_completed', 'Sipariş tamamlandı'),
      field('order_completed', 'cart_id', 'Siparişin sepet numarası'),
    ],
    brief:
      'Sepet terk edildiğinde (cart_abandoned) müşteri akışa girsin. Sepet tutarı (event.properties.amount) 50’nin altındaysa uygun değil olarak çık. 2 saat bekle. Akışın herhangi bir anında aynı sepet için sipariş (order_completed, cart_id eşleşmesi) gelirse hemen dönüşüm olarak çık; başka bir sepetin siparişi akışı bitirmez. 2 saat sonunda sipariş yoksa SEPET15 indirim kodunu sunan bir e-posta gönder. Sonra 24 saat bekle; hâlâ sipariş yoksa ve müşterinin SMS izni varsa aynı kodu içeren kısa bir SMS gönder ve normal tamamla; SMS izni yoksa SMS göndermeden normal tamamla. Bağlantı olarak https://example.com/sepet kullan.' +
      LANG,
    entry: 'cart_abandoned',
    props: { cart_id: 'cart-1', amount: 80 },
    thresholds: [{ path: 'event.properties.amount', operator: 'greater_than_or_equal', value: 50 }],
    cases: [
      {
        name: 'No order',
        contract: true,
        email: 1,
        sms: 1,
        outcome: 'completed',
        duration: 26 * H,
      },
      {
        name: 'Same cart ordered within two hours',
        contract: true,
        events: [{ name: 'order_completed', after: H, binding: 'cart_id' }],
        email: 0,
        sms: 0,
        outcome: 'converted',
        duration: H,
      },
      {
        name: 'No SMS consent',
        contract: true,
        consents: { sms: false },
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 26 * H,
      },
      {
        name: 'Below threshold',
        contract: true,
        inputs: { amount: 30 },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Exactly 50 qualifies',
        inputs: { amount: 50 },
        email: 1,
        sms: 1,
        outcome: 'completed',
      },
      {
        name: 'Another cart ordered',
        events: [{ name: 'order_completed', after: H, props: { cart_id: 'cart-other' } }],
        email: 1,
        sms: 1,
        outcome: 'completed',
      },
      {
        name: 'Same cart ordered after the email',
        events: [{ name: 'order_completed', after: 10 * H, binding: 'cart_id' }],
        email: 1,
        sms: 0,
        outcome: 'converted',
      },
    ],
  },
  {
    id: 'welcome_click',
    name: 'Hoş geldin – tıklama bekle',
    feature: 'wait for a click on an earlier email (message_node)',
    catalog: [firstName, ev('user_signed_up', 'Üye oldu')],
    brief:
      'Yeni üye olan (user_signed_up) müşteri akışa girsin. E-posta izni yoksa hemen uygun değil olarak çık. Hemen https://example.com/basla bağlantılı bir hoş geldin e-postası gönder. Bu e-postadaki bir bağlantıya 3 gün içinde tıklanırsa dönüşüm olarak çık. 3 gün içinde tıklanmazsa ikinci bir hatırlatma e-postası gönder ve normal tamamla. SMS hiç gönderme.' +
      LANG,
    entry: 'user_signed_up',
    props: {},
    cases: [
      {
        name: 'Never clicks',
        contract: true,
        email: 2,
        sms: 0,
        outcome: 'completed',
        duration: 3 * D,
      },
      {
        name: 'Clicks the welcome email',
        contract: true,
        events: [{ name: 'message_clicked', after: H }],
        email: 1,
        sms: 0,
        outcome: 'converted',
        duration: H,
      },
      {
        name: 'No email consent',
        contract: true,
        consents: { email: false },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Opens but does not click',
        events: [{ name: 'message_opened', after: H }],
        email: 2,
        sms: 0,
        outcome: 'completed',
      },
      {
        name: 'Click names the welcome step',
        events: [{ name: 'message_clicked', after: 2 * H, node: 'first_email' }],
        email: 1,
        sms: 0,
        outcome: 'converted',
      },
    ],
  },
  {
    id: 'winback_code',
    name: 'Geri kazanım – GERIDON20',
    feature: 'discount code in email, customer-wide goal exit',
    code: 'GERIDON20',
    catalog: [
      firstName,
      ev('subscription_cancelled', 'Abonelik iptal edildi'),
      ev('subscription_restarted', 'Abonelik yeniden başladı'),
    ],
    brief:
      'Abonelik iptal edildiğinde (subscription_cancelled) akış başlasın. 7 gün bekle, sonra GERIDON20 kodunu sunan bir geri dönüş e-postası gönder (https://example.com/geri-don). Ardından 14 gün bekle ve normal tamamla. Akışın herhangi bir anında abonelik yeniden başlarsa (subscription_restarted) hemen dönüşüm olarak çık. SMS gönderme.' +
      LANG,
    entry: 'subscription_cancelled',
    props: {},
    cases: [
      {
        name: 'Does not return',
        contract: true,
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 21 * D,
      },
      {
        name: 'Returns before the email',
        contract: true,
        events: [{ name: 'subscription_restarted', after: 2 * D }],
        email: 0,
        sms: 0,
        outcome: 'converted',
        duration: 2 * D,
      },
      {
        name: 'Returns after the email',
        contract: true,
        events: [{ name: 'subscription_restarted', after: 10 * D }],
        email: 1,
        sms: 0,
        outcome: 'converted',
        duration: 10 * D,
      },
      {
        name: 'Returns on the last day',
        events: [{ name: 'subscription_restarted', after: 20 * D }],
        email: 1,
        sms: 0,
        outcome: 'converted',
      },
    ],
  },
  {
    id: 'browse_open',
    name: 'İncelenen ürün – açılma bekle',
    feature: 'wait for an open of an earlier email, SMS fallback with consent',
    catalog: [
      firstName,
      ev('product_viewed', 'Ürün incelendi'),
      field('product_viewed', 'product_id', 'Ürün numarası'),
      field('product_viewed', 'product_name', 'Ürün adı'),
    ],
    brief:
      'Müşteri bir ürünü incelediğinde (product_viewed) akışa girsin. E-posta izni yoksa hemen uygun değil olarak çık. 1 saat bekle ve incelediği ürünü hatırlatan bir e-posta gönder (ürün adı event.properties.product_name, yoksa "incelediğin ürün"). Sonra 48 saat bu e-postanın açılmasını bekle. Açılırsa başka mesaj göndermeden normal tamamla. Açılmazsa ve SMS izni varsa https://example.com/urun bağlantılı kısa bir SMS hatırlatması gönder ve normal tamamla; SMS izni yoksa SMS göndermeden normal tamamla.' +
      LANG,
    entry: 'product_viewed',
    props: { product_id: 'p-9', product_name: 'Yürüyüş botu' },
    cases: [
      {
        name: 'Never opens',
        contract: true,
        email: 1,
        sms: 1,
        outcome: 'completed',
        duration: 49 * H,
      },
      {
        name: 'Opens the email',
        contract: true,
        events: [{ name: 'message_opened', after: 2 * H }],
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 2 * H,
      },
      {
        name: 'No SMS consent',
        contract: true,
        consents: { sms: false },
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 49 * H,
      },
      {
        name: 'No email consent',
        contract: true,
        consents: { email: false },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Clicks without an open',
        events: [{ name: 'message_clicked', after: 2 * H }],
        email: 1,
        sms: 1,
        outcome: 'completed',
      },
    ],
  },
  {
    id: 'review_request',
    name: 'Değerlendirme isteği',
    feature: 'order-scoped goal exit (correlation)',
    catalog: [
      firstName,
      ev('order_delivered', 'Sipariş teslim edildi'),
      field('order_delivered', 'order_id', 'Sipariş numarası'),
      ev('review_submitted', 'Değerlendirme yapıldı'),
      field('review_submitted', 'order_id', 'Değerlendirilen sipariş'),
    ],
    brief:
      'Sipariş teslim edildiğinde (order_delivered) akış başlasın. 3 gün bekle ve bu sipariş için değerlendirme isteyen bir e-posta gönder (https://example.com/degerlendir). Sonra 7 gün bekle ve normal tamamla. Aynı sipariş için değerlendirme (review_submitted, order_id eşleşmesi) gelirse akışın herhangi bir anında hemen dönüşüm olarak çık; başka bir siparişin değerlendirmesi bu akışı etkilemez. SMS gönderme.' +
      LANG,
    entry: 'order_delivered',
    props: { order_id: 'o-1' },
    cases: [
      {
        name: 'No review',
        contract: true,
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 10 * D,
      },
      {
        name: 'Reviews after the email',
        contract: true,
        events: [{ name: 'review_submitted', after: 4 * D, binding: 'order_id' }],
        email: 1,
        sms: 0,
        outcome: 'converted',
        duration: 4 * D,
      },
      {
        name: 'Reviews before the email',
        contract: true,
        events: [{ name: 'review_submitted', after: D, binding: 'order_id' }],
        email: 0,
        sms: 0,
        outcome: 'converted',
        duration: D,
      },
      {
        name: 'Reviews another order',
        events: [{ name: 'review_submitted', after: D, props: { order_id: 'o-2' } }],
        email: 1,
        sms: 0,
        outcome: 'completed',
      },
    ],
  },
  {
    id: 'loyalty_gold',
    name: 'Sadakat – altın üyelik',
    feature: 'inclusive threshold, attribute update',
    catalog: [
      firstName,
      customer('tier', 'Üyelik seviyesi'),
      ev('points_updated', 'Puan güncellendi'),
      field('points_updated', 'points', 'Toplam puan', 'number'),
    ],
    brief:
      'Puan güncellendiğinde (points_updated) akış başlasın. Toplam puan (event.properties.points) 1000 veya üzerindeyse müşterinin tier özelliğini "gold" yap, "Altın üyeliğe geçtin" e-postası gönder (https://example.com/altin) ve normal tamamla. 1000’in altındaysa hiçbir şey yapmadan uygun değil olarak çık. SMS gönderme.' +
      LANG,
    entry: 'points_updated',
    props: { points: 1500 },
    thresholds: [
      { path: 'event.properties.points', operator: 'greater_than_or_equal', value: 1000 },
    ],
    cases: [
      {
        name: 'Reaches gold',
        contract: true,
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 0,
        attributes: { tier: 'gold' },
      },
      {
        name: 'Below 1000',
        contract: true,
        inputs: { points: 999 },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Exactly 1000',
        inputs: { points: 1000 },
        email: 1,
        sms: 0,
        outcome: 'completed',
        attributes: { tier: 'gold' },
      },
      {
        name: '999.5 stays out',
        inputs: { points: 999.5 },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
      },
    ],
  },
  {
    id: 'price_drop',
    name: 'Fiyat düştü – kanal seçimi',
    feature: 'channel choice by consent, threshold',
    catalog: [
      firstName,
      ev('price_dropped', 'Fiyat düştü'),
      field('price_dropped', 'product_id', 'Ürün numarası'),
      field('price_dropped', 'discount_percent', 'İndirim oranı', 'number'),
    ],
    brief:
      'Takip edilen bir ürünün fiyatı düştüğünde (price_dropped) akış başlasın. İndirim oranı (event.properties.discount_percent) 20’nin altındaysa uygun değil olarak çık. 20 veya üzerindeyse: SMS izni varsa yalnızca kısa bir SMS gönder; SMS izni yoksa ama e-posta izni varsa yalnızca bir e-posta gönder; ikisi de yoksa uygun değil olarak çık. Mesajdan sonra normal tamamla. Bağlantı olarak https://example.com/indirim kullan.' +
      LANG,
    entry: 'price_dropped',
    props: { product_id: 'p-3', discount_percent: 25 },
    thresholds: [
      { path: 'event.properties.discount_percent', operator: 'greater_than_or_equal', value: 20 },
    ],
    cases: [
      { name: 'SMS allowed', contract: true, email: 0, sms: 1, outcome: 'completed', duration: 0 },
      {
        name: 'Email only',
        contract: true,
        consents: { sms: false },
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 0,
      },
      {
        name: 'No channel',
        contract: true,
        consents: { sms: false, email: false },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Small discount',
        contract: true,
        inputs: { discount_percent: 10 },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Exactly 20 percent',
        inputs: { discount_percent: 20 },
        email: 0,
        sms: 1,
        outcome: 'completed',
      },
    ],
  },
  {
    id: 'invoice_reminder',
    name: 'Fatura hatırlatma',
    feature: 'invoice-scoped exit, staged email then SMS',
    catalog: [
      firstName,
      ev('invoice_issued', 'Fatura kesildi'),
      field('invoice_issued', 'invoice_id', 'Fatura numarası'),
      ev('invoice_paid', 'Fatura ödendi'),
      field('invoice_paid', 'invoice_id', 'Ödenen fatura'),
    ],
    brief:
      'Fatura kesildiğinde (invoice_issued) akış başlasın. Aynı fatura ödendiğinde (invoice_paid, invoice_id eşleşmesi) akışın her yerinde hemen dönüşüm olarak çık; başka bir faturanın ödemesi bu akışı bitirmez. 5 gün bekle; ödenmemişse hatırlatma e-postası gönder. Sonra 2 gün daha bekle; hâlâ ödenmemişse ve SMS izni varsa kısa bir SMS hatırlatması gönder ve normal tamamla; SMS izni yoksa SMS göndermeden normal tamamla. Bağlantı olarak https://example.com/fatura kullan.' +
      LANG,
    entry: 'invoice_issued',
    props: { invoice_id: 'inv-1' },
    cases: [
      {
        name: 'Never paid',
        contract: true,
        email: 1,
        sms: 1,
        outcome: 'completed',
        duration: 7 * D,
      },
      {
        name: 'Paid on day one',
        contract: true,
        events: [{ name: 'invoice_paid', after: D, binding: 'invoice_id' }],
        email: 0,
        sms: 0,
        outcome: 'converted',
        duration: D,
      },
      {
        name: 'Paid after the email',
        contract: true,
        events: [{ name: 'invoice_paid', after: 6 * D, binding: 'invoice_id' }],
        email: 1,
        sms: 0,
        outcome: 'converted',
        duration: 6 * D,
      },
      {
        name: 'Another invoice paid',
        events: [{ name: 'invoice_paid', after: D, props: { invoice_id: 'inv-2' } }],
        email: 1,
        sms: 1,
        outcome: 'completed',
      },
      { name: 'No SMS consent', consents: { sms: false }, email: 1, sms: 0, outcome: 'completed' },
    ],
  },
  {
    id: 'webinar_click',
    name: 'Webinar – SMS tıklaması',
    feature: 'wait for a click on an earlier SMS (message_node)',
    catalog: [
      firstName,
      ev('webinar_registered', 'Webinara kayıt oldu'),
      field('webinar_registered', 'webinar_id', 'Webinar numarası'),
    ],
    brief:
      'Webinara kayıt olunduğunda (webinar_registered) akış başlasın. 1 gün bekle ve hatırlatma e-postası gönder. 1 gün daha bekle; SMS izni varsa https://example.com/canli katılım bağlantılı "yayın başlıyor" SMS’i gönder. Bu SMS’teki bağlantıya 2 saat içinde tıklanırsa dönüşüm olarak çık, tıklanmazsa normal tamamla. SMS izni yoksa SMS’i atla ve normal tamamla.' +
      LANG,
    entry: 'webinar_registered',
    props: { webinar_id: 'w-7' },
    cases: [
      {
        name: 'Does not click',
        contract: true,
        email: 1,
        sms: 1,
        outcome: 'completed',
        duration: 50 * H,
      },
      {
        name: 'Clicks the SMS',
        contract: true,
        events: [{ name: 'message_clicked', after: 48 * H + 600 }],
        email: 1,
        sms: 1,
        outcome: 'converted',
        duration: 48 * H + 600,
      },
      {
        name: 'No SMS consent',
        contract: true,
        consents: { sms: false },
        email: 1,
        sms: 0,
        outcome: 'completed',
        duration: 48 * H,
      },
      {
        name: 'Clicks only the email',
        events: [{ name: 'message_clicked', after: 25 * H }],
        email: 1,
        sms: 1,
        outcome: 'completed',
      },
    ],
  },
  {
    id: 'restock_sms',
    name: 'Stok geldi – STOK10 SMS',
    feature: 'SMS-only with code, event wait with timeout',
    code: 'STOK10',
    catalog: [
      firstName,
      ev('stock_alert_requested', 'Stok bildirimi istendi'),
      field('stock_alert_requested', 'product_id', 'Ürün numarası'),
      ev('product_restocked', 'Ürün stoğa girdi'),
      field('product_restocked', 'product_id', 'Stoğa giren ürün'),
    ],
    brief:
      'Müşteri stok bildirimi istediğinde (stock_alert_requested) akış başlasın. SMS izni yoksa hemen uygun değil olarak çık; e-posta hiç gönderme. 14 gün boyunca ürünün stoğa girmesini (product_restocked) bekle. Gelirse STOK10 kodunu içeren kısa bir SMS gönder (https://example.com/stok) ve normal tamamla. Süre dolarsa mesaj göndermeden normal tamamla. Entegrasyon stok olayını yalnızca müşterinin beklediği ürün için üretir.' +
      LANG,
    entry: 'stock_alert_requested',
    props: { product_id: 'p-5' },
    cases: [
      {
        name: 'Restocked on day two',
        contract: true,
        events: [{ name: 'product_restocked', after: 2 * D, binding: 'product_id' }],
        email: 0,
        sms: 1,
        outcome: 'completed',
        duration: 2 * D,
      },
      {
        name: 'Never restocked',
        contract: true,
        email: 0,
        sms: 0,
        outcome: 'completed',
        duration: 14 * D,
      },
      {
        name: 'No SMS consent',
        contract: true,
        consents: { sms: false },
        email: 0,
        sms: 0,
        outcome: 'condition_not_met',
        duration: 0,
      },
      {
        name: 'Restocked on day thirteen',
        events: [{ name: 'product_restocked', after: 13 * D, binding: 'product_id' }],
        email: 0,
        sms: 1,
        outcome: 'completed',
      },
    ],
  },
];

// ---------- contract and scenario builders ----------
const prefixed = (inputs = {}) =>
  Object.fromEntries(Object.entries(inputs).map(([k, v]) => ['event.properties.' + k, v]));
// A bound event's private example carries the entry's identity, so the
// example agrees with the reviewed relationship.
const contractEvents = (events = [], props = {}) =>
  events.map((e) => ({
    event_name: e.name,
    after_seconds: e.after,
    ...(e.binding ? { entry_binding: bind(e.binding) } : {}),
    ...(e.binding || e.props
      ? { properties: { ...(e.binding ? { [e.binding]: props[e.binding] } : {}), ...e.props } }
      : {}),
  }));
function contract(s) {
  const [main, ...rest] = s.cases.filter((c) => c.contract);
  const clause = (c) => ({
    ...(c.inputs ? { inputs: prefixed(c.inputs) } : {}),
    ...(c.consents ? { consents: c.consents } : {}),
    ...(c.events ? { events: contractEvents(c.events, { ...s.props, ...c.inputs }) } : {}),
    email_count: c.email,
    sms_count: c.sms,
    outcome: c.outcome,
    ...(c.duration !== undefined ? { duration_seconds: c.duration } : {}),
    ...(c.attributes ? { expected_attributes: c.attributes } : {}),
  });
  return {
    version: 2,
    confirmed: true,
    entry_event: s.entry,
    case_name: main.name,
    ...clause(main),
    inputs: {
      'customer.attributes.first_name': 'Deniz',
      ...prefixed(s.props),
      ...prefixed(main.inputs),
    },
    ...(s.thresholds ? { thresholds: s.thresholds, ineligible_outcome: 'condition_not_met' } : {}),
    alternatives: rest.map((c) => ({ name: c.name, ...clause(c) })),
  };
}
function scenario(s, c, i, workflow) {
  const props = { ...s.props, ...(c.inputs || {}) };
  const firstEmail = workflow.nodes.find((n) => n.type === 'email')?.id;
  return {
    name: `${String.fromCharCode(65 + i)} · ${c.name}`,
    start_time: START,
    customer: {
      customer_id: 'fictional_' + s.id,
      email: 'deniz@example.invalid',
      phone: '+15005550006',
      attributes: { first_name: 'Deniz' },
      consents: { email: true, sms: true, ...(c.consents || {}) },
    },
    entry_event: {
      event_id: 'entry_' + s.id,
      event_name: s.entry,
      occurred_at: START,
      properties: props,
    },
    events: (c.events || []).map((e, j) => ({
      after_seconds: e.after,
      event: {
        event_id: `followup_${s.id}_${i}_${j}`,
        event_name: e.name,
        properties: {
          ...(e.binding ? { [e.binding]: props[e.binding] } : {}),
          ...(e.props || {}),
          ...(e.node === 'first_email' ? { node_id: firstEmail } : {}),
        },
      },
    })),
    frequency_limit: 20,
    frequency_window_seconds: 86400,
    assertions: [
      { type: 'message_count', channel: 'email', expected: c.email },
      { type: 'message_count', channel: 'sms', expected: c.sms },
      { type: 'no_duplicate_delivery', expected: true },
    ],
  };
}

// ---------- static checks on the generated graph ----------
function staticChecks(s, w) {
  const notes = [];
  const messages = w.nodes.filter((n) => n.type === 'email' || n.type === 'sms');
  const waits = w.nodes.filter((n) => n.type === 'wait_event');
  if (s.code) {
    const offering = messages.filter((n) => n.config.discount_code);
    if (!offering.length) notes.push(`no message sets discount_code ${s.code}`);
    for (const n of offering) {
      if (n.config.discount_code !== s.code)
        notes.push(`${n.id} code ${n.config.discount_code} ≠ ${s.code}`);
      if (n.type === 'sms' && !/\{\{\s*offer\.code\s*\}\}/.test(n.config.text || ''))
        notes.push(`${n.id} SMS does not show {{ offer.code }}`);
    }
    for (const n of messages)
      if (
        (JSON.stringify(n.config).match(new RegExp(s.code, 'g')) || []).length >
        (n.config.discount_code === s.code ? 1 : 0)
      )
        notes.push(`${n.id} types the code out instead of {{ offer.code }}`);
  } else {
    for (const n of messages)
      if (n.config.discount_code) notes.push(`${n.id} invents code ${n.config.discount_code}`);
  }
  for (const n of waits) {
    const engagement = n.config.event_name?.startsWith('message_');
    if (engagement && !n.config.message_node)
      notes.push(`${n.id} waits on engagement without message_node`);
    if (!engagement && n.config.message_node)
      notes.push(`${n.id} sets message_node on a business event`);
    if (n.config.message_node && !messages.some((m) => m.id === n.config.message_node))
      notes.push(`${n.id} message_node ${n.config.message_node} is not a message step`);
  }
  if (s.id === 'welcome_click') {
    const wait = waits.find((n) => n.config.event_name === 'message_clicked');
    const first = messages[0];
    if (!wait) notes.push('no wait for message_clicked');
    else if (first && wait.config.message_node !== first.id)
      notes.push(
        `click wait targets ${wait.config.message_node}, not the welcome email ${first.id}`,
      );
  }
  if (s.id === 'browse_open' && !waits.some((n) => n.config.event_name === 'message_opened'))
    notes.push('no wait for message_opened');
  if (s.id === 'webinar_click') {
    const wait = waits.find((n) => n.config.event_name === 'message_clicked');
    const sms = messages.find((n) => n.type === 'sms');
    if (!wait || !sms || wait.config.message_node !== sms.id)
      notes.push('the click wait does not target the SMS step');
  }
  if (s.id === 'restock_sms' && messages.some((n) => n.type === 'email')) notes.push('sends email');
  return notes;
}

// ---------- HTTP ----------
function client() {
  let cookie = '';
  let csrf = '';
  return {
    set csrf(v) {
      csrf = v;
    },
    async call(path, method = 'GET', data, timeout = 20000) {
      assert(
        !/\/publish$|\/messages\/test-send|\/lifecycle|\/(pause|resume|sunset|complete|archive)$|\/audience\/runs/.test(
          path,
        ),
        'Publishing, sends and lifecycle changes are not part of this run: ' + path,
      );
      const started = performance.now();
      const response = await fetch(new URL('/api/v1' + path, base), {
        method,
        headers: { 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf },
        body: data === undefined ? undefined : JSON.stringify(data),
        signal: AbortSignal.timeout(timeout),
      });
      if (response.headers.has('set-cookie'))
        cookie = response.headers.get('set-cookie').split(';')[0];
      const text = await response.text();
      let body;
      try {
        body = JSON.parse(text);
      } catch {
        body = text;
      }
      return { status: response.status, body, ms: Math.round(performance.now() - started) };
    },
  };
}
const ok = (r, what) => {
  if (r.status >= 300)
    throw new Error(`${what}: HTTP ${r.status} ${JSON.stringify(r.body).slice(0, 300)}`);
  return r.body;
};
const sql = (query) =>
  execFileSync(
    'docker',
    [
      'compose',
      'exec',
      '-T',
      'postgres',
      'psql',
      '-U',
      'journey',
      '-d',
      'journeys',
      '-At',
      '-c',
      query,
    ],
    {
      encoding: 'utf8',
    },
  ).trim();

// ---------- run ----------
async function runScenario(s) {
  const record = { id: s.id, name: s.name, feature: s.feature, brief: s.brief, catalog: s.catalog };
  const c = client();
  const user = ok(
    await c.call('/auth/register', 'POST', {
      name: 'Live scenario reviewer',
      workspace: 'Live · ' + s.name,
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    }),
    'register',
  );
  c.csrf = user.csrf_token;
  record.workspace_id = user.workspace_id;
  ok(await c.call('/data-catalog', 'PUT', { entries: s.catalog }), 'catalog');
  record.behavior_checks = contract(s);

  const empty = { schema_version: 1, nodes: [], edges: [] };
  const input = {
    behavior_checks: record.behavior_checks,
    provider: 'openai',
    action: 'generate',
    prompt: s.brief,
    name: s.name,
    description: '',
    workflow: empty,
  };
  record.generation_requests = [];
  let proposal;
  for (let attempt = 1; attempt <= 2 && !proposal; attempt++) {
    const r = await c.call('/ai/journeys/generate', 'POST', input, 215000);
    record.generation_requests.push({
      attempt,
      http_status: r.status,
      ms: r.ms,
      error: r.status >= 300 ? r.body?.detail || r.body : undefined,
    });
    if (r.status < 300) proposal = r.body;
    else if (![429, 502, 504].includes(r.status)) break;
  }
  if (!proposal) {
    record.verdict = 'generation_failed';
    return record;
  }
  record.generation = proposal.generation;
  record.proposal = {
    name: proposal.name,
    explanation: proposal.explanation,
    warnings: proposal.warnings,
    grounding_ready: proposal.grounding?.ready,
    not_done: proposal.not_done || [],
    workflow: proposal.workflow,
  };
  const w = proposal.workflow;
  record.static_notes = staticChecks(s, w);

  // A human applies the proposal to a draft unchanged.
  const journey = ok(
    await c.call('/journeys', 'POST', { name: proposal.name || s.name, creation_step: 'builder' }),
    'journey',
  );
  record.journey_id = journey.id;
  const saved = ok(
    await c.call(`/journeys/${journey.id}/draft`, 'PUT', {
      name: proposal.name || s.name,
      description: proposal.description || '',
      draft: w,
      revision: journey.revision,
    }),
    'draft',
  );
  record.validation = ok(await c.call(`/journeys/${journey.id}/validate`, 'POST', {}), 'validate');

  record.cases = [];
  for (const [i, k] of s.cases.entries()) {
    const sc = scenario(s, k, i, w);
    const run = ok(
      await c.call('/test-runs', 'POST', {
        journey_id: journey.id,
        use_current_draft: true,
        scenario: sc,
      }),
      'test run',
    );
    const done = ok(
      await c.call(`/test-runs/${run.id}/advance`, 'POST', {
        operation: 'complete',
        revision: run.revision,
      }),
      'advance',
    );
    const st = done.state;
    const accepted = (st.messages || []).filter((m) => m.accepted_at && m.execution === 1);
    const email = accepted.filter((m) => m.channel === 'email').length;
    const sms = accepted.filter((m) => m.channel === 'sms').length;
    const ends = (st.steps || [])
      .filter((x) => x.execution === 1)
      .map((x) => Date.parse(x.completed_at || x.started_at));
    const duration = ends.length
      ? Math.round((Math.max(...ends) - Date.parse(START)) / 1000)
      : null;
    const outcome =
      st.enrollment_count === 0 ? 'not_enrolled' : st.exit_reason || st.status.toLowerCase();
    const outcomeOk =
      outcome === k.outcome || (k.outcome === 'condition_not_met' && outcome === 'not_enrolled');
    const attributes = k.attributes
      ? Object.entries(k.attributes).every(([a, v]) => st.customer?.attributes?.[a] === v)
      : true;
    // Sandbox state keeps message metadata; the rendered bodies are in the test inbox.
    const inbox = ok(await c.call(`/test-inbox?test_run_id=${run.id}&status=SIMULATED`), 'inbox');
    const bodies = [];
    for (const item of inbox)
      bodies.push(ok(await c.call(`/test-messages/${item.id}`), 'test message'));
    const offering = new Set(w.nodes.filter((n) => n.config?.discount_code).map((n) => n.id));
    const codeShown = bodies.every((b) => {
      const text = JSON.stringify(b.message);
      return offering.has(b.source_node) ? text.includes(s.code) : !/offer\.code/.test(text);
    });
    if (!record.sample_messages) record.sample_messages = {};
    for (const b of bodies)
      record.sample_messages[b.source_node] ??= {
        channel: b.channel,
        subject: b.message?.subject,
        text: b.message?.text,
      };
    const pass =
      email === k.email &&
      sms === k.sms &&
      outcomeOk &&
      attributes &&
      codeShown &&
      st.status !== 'FAILED';
    record.cases.push({
      name: sc.name,
      held_out: !k.contract,
      test_run_id: run.id,
      expected: { email: k.email, sms: k.sms, outcome: k.outcome, duration: k.duration ?? null },
      actual: { email, sms, outcome, duration, status: st.status },
      duration_matches: k.duration === undefined ? null : duration === k.duration,
      attributes_ok: attributes,
      code_shown: codeShown,
      inbox_messages: bodies.length,
      pass,
    });
  }
  record.publish_check = ok(await c.call(`/journeys/${journey.id}/publish-check`), 'publish-check');
  const counts = sql(
    `SELECT json_build_object('journeys',(SELECT count(*) FROM journeys WHERE workspace_id='${user.workspace_id}'),'versions',(SELECT count(*) FROM journey_versions WHERE workspace_id='${user.workspace_id}'),'live_executions',(SELECT count(*) FROM journey_executions WHERE workspace_id='${user.workspace_id}'),'live_customers',(SELECT count(*) FROM customers WHERE workspace_id='${user.workspace_id}'),'delivery_jobs',(SELECT count(*) FROM delivery_jobs WHERE workspace_id='${user.workspace_id}'),'test_runs',(SELECT count(*) FROM test_runs WHERE workspace_id='${user.workspace_id}'),'draft_status',(SELECT status FROM journeys WHERE id='${journey.id}'))`,
  );
  record.database_readback = JSON.parse(counts);
  const passed = record.cases.filter((x) => x.pass).length;
  record.summary = {
    cases: record.cases.length,
    passed,
    contract_passed: record.cases.filter((x) => !x.held_out && x.pass).length,
    contract_cases: record.cases.filter((x) => !x.held_out).length,
    held_out_passed: record.cases.filter((x) => x.held_out && x.pass).length,
    held_out_cases: record.cases.filter((x) => x.held_out).length,
    durations_matched: record.cases.filter((x) => x.duration_matches === true).length,
    durations_checked: record.cases.filter((x) => x.duration_matches !== null).length,
    static_notes: record.static_notes.length,
    publish_ready: record.publish_check.ready,
  };
  record.verdict =
    passed === record.cases.length && !record.static_notes.length && record.publish_check.ready
      ? 'pass'
      : 'issues';
  void saved;
  return record;
}

const settings = await (async () => {
  const c = client();
  const user = ok(
    await c.call('/auth/register', 'POST', {
      name: 'Live settings check',
      workspace: 'Live · settings',
      email: randomBytes(12).toString('hex') + '@example.invalid',
      password: randomBytes(24).toString('hex'),
    }),
    'register',
  );
  c.csrf = user.csrf_token;
  return ok(await c.call('/settings/ai'), 'settings');
})();
console.log('AI settings:', JSON.stringify(settings));
const commit = execFileSync('git', ['rev-parse', '--short', 'HEAD'], { encoding: 'utf8' }).trim();
const result = {
  started_at: new Date().toISOString(),
  source_commit: commit,
  settings,
  journeys: [],
};
await mkdir(out, { recursive: true });
for (const s of scenarios.filter((x) => !only || only.includes(x.id))) {
  const started = performance.now();
  let record;
  try {
    record = await runScenario(s);
  } catch (error) {
    record = { id: s.id, name: s.name, verdict: 'error', error: String(error.message || error) };
  }
  record.wall_seconds = Math.round((performance.now() - started) / 100) / 10;
  result.journeys.push(record);
  const sum = record.summary;
  console.log(
    `${s.id.padEnd(17)} ${record.verdict.padEnd(17)} ${sum ? `${sum.passed}/${sum.cases} cases, contract ${sum.contract_passed}/${sum.contract_cases}, held-out ${sum.held_out_passed}/${sum.held_out_cases}, notes ${sum.static_notes}, ready ${sum.publish_ready}` : record.error || ''} ${record.wall_seconds}s`,
  );
  await writeFile(`${out}/result.json`, JSON.stringify(result, null, 2) + '\n');
}
result.finished_at = new Date().toISOString();
await writeFile(`${out}/result.json`, JSON.stringify(result, null, 2) + '\n');
