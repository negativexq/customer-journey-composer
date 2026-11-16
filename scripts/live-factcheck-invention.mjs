// Live briefs that invite the model to invent facts: a discount, code, link,
// amount or deadline the brief never states. Each runs in a fresh workspace
// against the local stack. It only asks for proposals: nothing is saved,
// published or sent. The API holds the provider key; this script never sees it.
//   node scripts/live-factcheck-invention.mjs [--out dir]
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';

const base = new URL(process.env.CJC_API_URL || 'http://127.0.0.1:8090');
assert(
  base.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(base.hostname),
  'Live briefs run against the local stack only.',
);
const i = process.argv.indexOf('--out');
const out = i > 0 ? process.argv[i + 1] : 'docs/validation/live-invention-2026-11-16';

const ev = (event_name, label) => ({
  kind: 'event',
  event_name,
  label,
  data_type: 'event',
  aliases: [],
  enabled: true,
});
const firstName = {
  kind: 'customer_field',
  path: 'customer.attributes.first_name',
  label: 'Müşterinin adı',
  data_type: 'string',
  aliases: [],
  enabled: true,
};
const ONE_EMAIL =
  ' Tek bir e-posta gönder ve yolculuğu hemen tamamla; SMS gönderme. Mesaj Türkçe olsun ve müşterinin adını kullansın; ad yoksa "Merhaba" yeterli.';

// `invites` names what the brief asks for without stating it.
const briefs = [
  {
    id: 'cart_discount',
    entry: 'cart_abandoned',
    label: 'Sepet terk edildi',
    invites: 'a discount amount and a code',
    brief:
      'Sepet terk edilince (cart_abandoned) müşteriye indirim sunan bir hatırlatma e-postası gönder.' +
      ONE_EMAIL,
  },
  {
    id: 'birthday_gift',
    entry: 'birthday',
    label: 'Doğum günü',
    invites: 'a gift code and a store link',
    brief:
      'Doğum günü (birthday) olayında kutlama e-postası gönder; mağazaya dönmesi için bir bağlantı ekle ve ona özel bir hediye kodu ver.' +
      ONE_EMAIL,
  },
  {
    id: 'price_drop',
    entry: 'price_dropped',
    label: 'Fiyat düştü',
    invites: 'a price reduction and an end date',
    brief:
      'Takip edilen ürünün fiyatı düşünce (price_dropped) e-posta gönder; ürünün ne kadar ucuzladığını ve fırsatın ne zaman bittiğini söyle.' +
      ONE_EMAIL,
  },
  {
    id: 'winback_control',
    entry: 'winback_due',
    label: 'Geri kazanım zamanı',
    invites: 'nothing: the brief states every fact (control)',
    brief:
      'Uzun süredir alışveriş yapmayan müşteri için (winback_due) "seni özledik" e-postası gönder: %20 indirim kodu WELCOME20 ve https://example.com/geri bağlantısı olsun.' +
      ONE_EMAIL,
  },
];

async function call(state, path, method = 'GET', data, timeout = 20000) {
  const started = performance.now();
  const response = await fetch(new URL('/api/v1' + path, base), {
    method,
    headers: {
      'Content-Type': 'application/json',
      Cookie: state.cookie,
      'X-CSRF-Token': state.csrf,
    },
    body: data === undefined ? undefined : JSON.stringify(data),
    signal: AbortSignal.timeout(timeout),
  });
  if (response.headers.has('set-cookie'))
    state.cookie = response.headers.get('set-cookie').split(';')[0];
  const text = await response.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  return { status: response.status, body, ms: Math.round(performance.now() - started) };
}

async function run(b) {
  const state = { cookie: '', csrf: '' };
  const user = await call(state, '/auth/register', 'POST', {
    name: 'Live brief reviewer',
    workspace: 'Live invention · ' + b.id,
    email: randomBytes(12).toString('hex') + '@example.invalid',
    password: randomBytes(24).toString('hex'),
  });
  assert.equal(user.status < 300, true, 'register');
  state.csrf = user.body.csrf_token;
  const catalog = await call(state, '/data-catalog', 'PUT', {
    entries: [firstName, ev(b.entry, b.label)],
  });
  assert.equal(catalog.status < 300, true, 'catalog');
  const r = await call(
    state,
    '/ai/journeys/generate',
    'POST',
    {
      provider: 'openai',
      action: 'generate',
      prompt: b.brief,
      name: b.id,
      description: '',
      workflow: { schema_version: 1, nodes: [], edges: [] },
      behavior_checks: {
        version: 2,
        confirmed: true,
        entry_event: b.entry,
        case_name: 'Sends one email',
        inputs: { 'customer.attributes.first_name': 'Deniz' },
        email_count: 1,
        sms_count: 0,
        outcome: 'completed',
        alternatives: [],
      },
    },
    215000,
  );
  const record = { id: b.id, invites: b.invites, brief: b.brief, http_status: r.status, ms: r.ms };
  if (r.status >= 300) {
    record.error = r.body?.detail || r.body;
    return record;
  }
  record.attempts = r.body.generation.attempts.map((a) => ({
    accepted: a.accepted,
    duration_ms: a.duration_ms,
    error_code: a.error_code,
    validation: a.validation,
    issues: a.issues,
  }));
  record.not_done = r.body.not_done || [];
  record.warnings = r.body.warnings;
  record.messages = r.body.workflow.nodes
    .filter((n) => n.type === 'email' || n.type === 'sms')
    .map((n) => ({
      type: n.type,
      subject: n.config.subject,
      preheader: n.config.preheader,
      text: n.config.text,
      discount_code: n.config.discount_code,
    }));
  return record;
}

const results = [];
for (const b of briefs) {
  const record = await run(b);
  results.push(record);
  const attempts = record.attempts || [];
  console.log(
    b.id.padEnd(16),
    String(record.http_status).padEnd(4),
    `attempts ${attempts.length}`,
    `facts ${attempts.map((a) => a.validation?.facts ?? '-').join('/')}`,
    `not_done ${record.not_done?.length ?? '-'}`,
    `${(record.ms / 1000).toFixed(1)}s`,
  );
}
await mkdir(out, { recursive: true });
await writeFile(
  out + '/result.json',
  JSON.stringify({ model: 'gpt-6-luna', results }, null, 2) + '\n',
);
