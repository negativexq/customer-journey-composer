# Customer Journey Composer

## Product Requirements & Implementation Plan — MVP v1.0

**Ürün:** AI-Powered Customer Journey Orchestration Platform

**Repo:** `customer-journey-composer`

**Öncelik:** Production-oriented MVP

**Ana prensip:** AI-generated, human-approved, deterministically executed.

**Gereksinim tarihi:** 2026-11-10

Bu belge kullanıcının MVP gereksinimlerini, kabul kriterlerini ve geliştirme sırasını kayıt altına alır. Uygulama durumu [IMPLEMENTATION.md](IMPLEMENTATION.md) içinde izlenir. Gereksinimlerin burada yer alması uygulanmış oldukları anlamına gelmez.

**Kullanıcı ek kararı:** AI katmanı hem Anthropic Claude API hem OpenAI API destekleyecek. Canlı LLM çağrılarıyla yapılan testler sonraya bırakılacak; deterministic testler, graph/schema validation ve test izolasyonu geliştirme sırasında yapılacak. AI olmadan elle oluşturma, yayınlama ve execution çalışabilmeli.

## 1. Ürün hedefi

Kullanıcı doğal dille journey oluşturur, görsel canvas'ta düzenler, müşteri profilleriyle test eder, onaylayarak yayınlar ve gerçek zamanlı izler. Ürün yalnızca workflow editor değildir: kalıcı durum yöneten, event işleyen ve mesaj gönderen execution engine içerir.

Temel akış:

1. Yeni journey veya template seçimi.
2. Doğal dilde otomasyon açıklaması.
3. Claude veya OpenAI ile geçerli workflow taslağı.
4. Canvas üzerinde node ve bağlantı düzenleme.
5. Email/SMS içeriklerinin hazırlanması.
6. Müşteri profilleriyle mesaj preview.
7. Test Lab içinde event ve zaman senaryoları.
8. Test Inbox içinde mesaj inceleme.
9. Validation ve insan onayıyla publish.
10. Gerçek müşteri event'leriyle execution.
11. Live Monitor ve Analytics ile izleme.

**MVP başarı tanımı:** Dışarıdan gelen `cart_updated` event'iyle başlayan, bekleyen, satın alma olmadığında mesaj gönderen ve satın alma olduğunda müşteriyi journey'den çıkaran otomasyon uçtan uca çalışır.

## 2. Uygulama ekranları

### 2.1 Overview Dashboard

Toplam journey; active/draft/paused/completed dağılımı; aktif execution; son 24 saat girişleri; gönderilen/engellenen/başarısız mesajlar; conversion sayısı ve oranı; son journey'ler; hatalı execution'lar; aktiviteler ve sistem uyarıları. Kartlar ilgili detaylara bağlanır.

### 2.2 Journeys

Ad, açıklama, etiketler, durum, sürüm, güncelleme zamanı; entry/active/completed/converted sayıları; arama, filtreleme, sıralama; oluşturma, duplicate, pause, resume, archive; template'den oluşturma ve detay bağlantısı.

### 2.3 Journey Detail

Sekmeler: **Builder**, **Preview & Test**, **Live Monitor**, **Analytics**, **Versions**. Save, Validate, Test, Publish, Pause, More Actions mevcut lifecycle durumuna göre kullanılabilir.

### 2.4 Customers

Liste ve arama; profil/custom attributes; event timeline; enrollment geçmişi; aktif execution; mesaj geçmişi; consent ve iletişim tercihleri.

### 2.5 Events

Son event'ler; tip, müşteri, timestamp, payload; validation durumu; duplicate/rejected bilgileri; manuel test event; JSON inceleme.

### 2.6 Test Lab ve Test Inbox

Ayrı fakat bağlantılı modüller. Test Lab senaryo oluşturur/çalıştırır. Test Inbox üretilen email/SMS çıktılarını gösterir.

### 2.7 Templates, Analytics, Settings

Templates: hazır ve kullanıcı journey şablonları. Analytics: karşılaştırmalar, trendler, kanal performansı. Settings: workspace, provider, consent, frequency cap, API key, kullanıcılar, audit logs.

## 3. Visual Journey Builder

**Vue Flow zorunludur.** Drag/drop node ekleme; bağlantı oluşturma/silme; sağ panelde node ayarları; zoom/pan/fit/minimap; undo/redo; copy/paste/duplicate; multi-select; otomatik yerleşim; autosave; klavye kısayolları; node validation; seçili node preview/test geçişi.

| Node | Yapılandırma | Çıkış |
| --- | --- | --- |
| Event Trigger | Event adı, filtreler, yeniden giriş politikası | Started |
| Wait / Delay | Süre, zaman dilimi | Completed |
| Wait for Event | Filtreler, timeout | Matched / Timeout |
| Condition | Alan, operatör, değer, AND/OR grupları | True / False |
| Send Email | Template, subject, content | Next |
| Send SMS | Template, content | Next |
| Update Attribute | Attribute, işlem, değer | Next |
| Exit | Reason/outcome | Completed |

Event Trigger örnekleri: `user_registered`, `cart_updated`, `purchase_completed`. Yeniden giriş: once, önceki tamamlandıktan sonra, cooldown sonrası. Varsayılan müşteri/journey başına en fazla bir aktif execution.

Wait saniye/dakika/saat/gün destekler; kalıcıdır; restart sonrası devam eder. Wait for Event müşteri event'i veya timeout ile dallanır; yarışta yalnızca biri kazanır.

Condition müşteri, event ve journey context üzerinde `equals`, `not equals`, `greater than`, `less than`, `contains`, `exists` ve tarih karşılaştırmaları destekler. AND/OR grupları bulunur.

Send Email/SMS kişiselleştirme ve eligibility sonrası delivery job oluşturur. Consent/frequency cap nedeniyle engellenen mesaj gönderilmiş sayılmaz. Update Attribute sınırlı `set`, `increment`, `unset` işlemleri yapar. Exit reason örnekleri: `completed`, `converted`, `condition_not_met`, `manually_cancelled`.

Validation: tek başlangıç; desteklenmeyen döngü yok; zorunlu portlar bağlı; dallar ve konfigürasyonlar geçerli; ulaşılamayan node tespiti; boyut limiti; en az bir tamamlanma yolu. Geçersiz workflow yayınlanamaz.

## 4. AI Journey Composer

Canvas yanında konuşma paneli. Örnek: “Yeni kayıt olan kullanıcılara hoş geldin e-postası gönder. 48 saat içinde satın alma yapmazlarsa SMS gönder. Satın alma gerçekleştiği anda journey'den çıkar.”

AI adı/açıklamayı, typed node ayarlarını, edge'leri, hedefleme/exit koşullarını, email/SMS taslaklarını ve eksik/belirsiz alan uyarılarını üretir.

- Anthropic Claude ve OpenAI provider adapter'ları; workspace/server ayarından seçim.
- Provider'ın desteklediği structured output; versiyonlu ortak JSON schema.
- Graph ve business validation; sınırlı otomatik düzeltme denemesi.
- Belirsizlikler kullanıcı onayı ister; generated değişiklikler diff olarak gösterilir.
- Aksiyonlar: Generate Journey, Modify Journey, Add Step, Explain Journey, Explain Node, Improve Message, Generate Message, Fix Validation Errors.
- Mevcut journey doğrudan kaydedilmez. Added/removed nodes, updated conditions, changed messages özeti ve **Apply / Discard**.
- AI runtime kurallarını değiştiremez; yalnızca izinli node specification üretir; kendiliğinden publish edemez.
- Customer payload güvenilmeyen veridir. PII'nin AI'a aktarılması MVP gereği değildir.
- Anahtarlar yalnızca backend environment üzerinden; tarayıcıya verilmez. Canlı LLM testleri daha sonra, kontrollü ve açıkça ayrı çalıştırılır.

## 5. Customer Profiles & Event Ingestion

Profil: `customer_id`, `external_id`, `email`, `phone`, `locale`, `timezone`, `attributes`, `consents`, `created_at`, `updated_at`. Custom attributes JSON; sık sorgulanan alanlar indekslenebilir.

```json
{
  "event_id": "evt_10001",
  "customer_id": "cust_123",
  "event_name": "cart_updated",
  "occurred_at": "2026-11-10T10:00:00Z",
  "properties": { "cart_id": "cart_500", "amount": 2450, "currency": "TRY" }
}
```

Single/batch ingestion; schema validation; workspace API key; unique event ID; duplicate detection; persistence; journey matching; timeline; açık payload hataları; tanımlı late-event politikası. Aynı event tekrar geldiğinde yeni logical execution/action oluşmaz.

## 6. Persistent Journey Execution Engine

**Teknik çekirdek.** Her müşteri için kalıcı execution.

Execution: `RUNNING`, `WAITING`, `COMPLETED`, `EXITED`, `FAILED`, `CANCELLED`. Step: `SCHEDULED`, `RUNNING`, `WAITING`, `SUCCEEDED`, `SKIPPED`, `FAILED`.

İşleme sırası:

1. Event alınır ve uygun aktif version'lar bulunur.
2. Entry/re-entry kontrol edilir; transaction içinde execution oluşturulur.
3. Başlangıç node'u işlenir; condition/wait durumu kalıcılaştırılır.
4. Zamanı gelen execution worker tarafından claim edilir.
5. Sonraki node seçilir; aksiyonlar idempotent planlanır.
6. Completion/global exit gerçekleşene kadar devam edilir.

PostgreSQL source of truth. Durable scheduled jobs; `SELECT ... FOR UPDATE SKIP LOCKED`; lease/claim token; stale lease recovery; atomik geçiş; transactional outbox; retry/backoff; failed jobs; graceful shutdown. Redis ancak gerçek cache/rate-limit ihtiyacında; workflow state Redis'e bağımlı değildir.

Invariant'lar:

- Execution aynı anda iki aktif node geçişi yapamaz.
- Event ID tekrar işlenmesi yeni logical action oluşturamaz.
- Execution/node aksiyonu başına tek logical delivery job.
- Published version immutable; execution başladığı version'da kalır.
- Event/timeout yarışında tek geçiş kazanır.
- Global exit bekleyen adımları iptal eder.
- Paused journey yeni müşteri almaz.
- Worker restart execution kaybettirmez.

Global exit örneği `purchase_completed`; müşteri wait sırasında satın alırsa execution sonlandırılır. Timer/exit yarışı transaction/state kontrolüyle çözülür.

Logical action ve job creation idempotent. Provider kabulünden sonra yanıt kaybolduğunda harici exactly-once teslim garanti edilmez. Provider destekliyorsa idempotency key; belirsiz sonuç ayrıca kayıt edilir.

## 7. Channel Studio

Email: subject, preheader, HTML/body, text fallback, CTA URL, değişkenler, template, desktop/mobile, Preview as Customer, Send Test. HTML sanitization, URL validation ve isolated preview zorunlu.

SMS: metin, personalization, karakter ve GSM-7/Unicode segment hesabı, test, phone preview, tahmini segment maliyeti.

Personalization: `Merhaba {{ customer.first_name }}, sepetindeki ürünler seni bekliyor.` Eksik alan fallback kullanır veya validation hatası verir; keyfi kod çalıştırılmaz.

Ortak email/SMS/mock/fault-injection adapter'ı; attempt history; bounded retry; timeout; provider response; idempotency key; message status. MVP: Mailpit email + mock SMS zorunlu; gerçek SMS opsiyonel.

## 8. Preview Studio

### 8.1 Message Preview

Seçili email için desktop/mobile, SMS phone mockup; subject/preheader; personalization values; CTA URL kontrolü; missing attributes; anlık template değişiklikleri.

### 8.2 Preview as Customer

Existing, New, Returning, Inactive, Custom Test Customer. Ad/soyad, sepet, satın alma geçmişi, locale/timezone, consent, custom attributes düzenlenir. Gerçek profil değiştirilmez.

### 8.3 Journey Path Preview

Girilen node'lar, condition true/false, oluşturulan mesajlar, beklenen adım/event, skip, gönderim zamanı, exit reason. Her adımda simulated timestamp. Gerçek provider çağrılmaz.

## 9. Test Lab

Scenario: version, test customer, başlangıç zamanı, entry event/payload, sonraki event'ler, attribute değişiklikleri, provider hata senaryosu, expected outcomes.

Virtual Clock: Run, Pause, Step, Advance 1h/1d, Run Until Next Event, Run to Completion, Reset, Replay. Runtime `Clock` soyutlaması kullanır; simulator kontrollü saat kullanır.

Assertions örnekleri: email var, SMS yok, completed, exit, condition false, duplicate delivery yok. Sonuç **PASS / FAIL**.

Türler: Single Node, Full Journey, Event Sequence, Failure Injection (429/500/timeout/duplicate/retry), Regression (kayıtlı senaryoyu yeni version ile karşılaştırma).

İzolasyon: test execution ayrı; gerçek kuyruğa girmez; production conversion etkilemez; canlı profili değiştirmez; sandbox provider dış çağrı yapmaz.

## 10. Test Inbox

Birinci sınıf modül. Sol: customers, runs, kanal/status filtreleri. Orta: mesajlar, journey, müşteri, simulated timestamp, node, status. Sağ: render, personalization, delivery decision, provider response, execution trace.

Durumlar: `SIMULATED`, `BLOCKED`, `SKIPPED`, `FAILED`, `TEST_ACCEPTED`; hangi test moduna ait olduğu görünür.

Detay: journey/version, execution/run ID, customer, source node, subject/body, rendered content, template ID, dynamic values, eligibility/frequency cap/consent, scheduled time, attempts, provider sonucu, trace ID.

Aksiyonlar: Open Preview, View Payload, View Execution Trace, Copy Rendered Content, Replay Scenario, Compare with Previous Run, customer/journey filtreleri.

Simulated inbox ile gerçek test gönderimi ayrıdır. Send Test Email açıkça belirlenmiş izinli test adresine SMTP ile; Send Test SMS MVP'de mock. Allowlist uygulanır; normal execution'dan ayrı işaretlenir.

## 11. Journey Test Scenarios

| Senaryo | Akış ve beklenen sonuç |
| --- | --- |
| A — Abandoned Cart | `cart_updated` → 2h wait → purchase condition → email → 24h purchase wait → SMS → complete. Email Test Inbox'ta. İlk 2h purchase: exit, mesaj yok. Timer/purchase yarışı: çift aksiyon ve exit sonrası send yok. |
| B — Onboarding | `user_registered` → welcome email → 48h wait → activation/purchase condition → aktif exit, inaktif SMS. |
| C — Duplicate Event | Aynı ID eşzamanlı iki kez: tek logical execution/action. |
| D — Worker Crash | Wait claim sonrası crash: lease recovery, güvenli devam, görev kaybı yok. |
| E — Provider Failure | 429 → 500 → success: backoff, bounded retry, attempt history. |
| F — Frequency Cap | İki journey aynı müşteriye: workspace cap, block reason görünür. |
| G — Version Update | v1 çalışırken v2 publish: mevcut v1, yeni giriş v2. |
| H — Test Isolation | 1.000 sandbox customer: production execution/send/analytics etkilenmez. |

## 12. Live Journey Monitor

Canvas node: entered, active/waiting, completed, skipped, failed, average processing/wait duration.

Track Customer ID ile path highlight: mevcut node, entry time, koşul, bekleme süresi/event, gönderilen ve engellenen mesaj/reason, exit time.

Execution Inspector: state, version, entry event, transition history, condition evaluation, wait deadline, jobs, attempts, errors/retries, correlation/trace ID. SSE veya kontrollü polling yeterli; WebSocket zorunlu değil.

## 13. Analytics & Reporting

Journey: total entries, unique customers, active/completed/converted executions, conversion rate, exit reasons, average completion time, node drop-off, channel performance.

Channel: scheduled, attempted, provider accepted, failed, skipped, clicked (varsa), opened (güvenilir ölçüm kadar). Provider accepted kesin delivery olarak raporlanmaz.

Grafikler: günlük entry/conversion, funnel, send/skip/failure, channel breakdown, journey comparison. Paydalar açık; retry yeni müşteri/logical send sayılmaz.

## 14. Journey Lifecycle & Safety

| Durum | Anlam |
| --- | --- |
| Draft | Düzenlenir, canlı execution almaz. |
| Active | Yeni girişler ve mevcut execution ilerler. |
| Paused | Yeni girişler ve yeni aksiyonlar durur; resume sonrası timer/continuation kuralı açıkça tanımlanır. |
| Sunset | Yeni giriş yok; mevcut execution devam eder. |
| Completed | Journey çalışmaz. |
| Archived | Tarihçe korunur; aktif execution yok. |

Test ayrı execution mode. Publish gate: graph, kanal, consent/eligibility, exit, frequency cap, değişkenler, provider, test sonuçları. Publish immutable snapshot oluşturur.

## 15. Veri modeli

| Tablo | Sorumluluk |
| --- | --- |
| workspaces | Çalışma alanı |
| users | Panel kullanıcıları |
| customers | Müşteri profilleri |
| customer_events | Gelen olaylar |
| journeys | Journey metadata |
| journey_versions | Immutable workflow |
| journey_executions | Müşteri execution |
| step_executions | Node geçmişi |
| scheduled_jobs | Kalıcı timer/görev |
| event_waiters | Beklenen event |
| message_templates | Kanal içerikleri |
| delivery_jobs | Logical send |
| delivery_attempts | Provider denemeleri |
| customer_consents | İletişim izinleri |
| test_scenarios | Kaydedilmiş senaryo |
| test_runs | Test execution |
| test_messages | Test Inbox |
| audit_logs | Yönetim aksiyonları |

Unique constraint ve indeksler migration seviyesinde; yalnızca application kontrolü yeterli değildir.

## 16. API kapsamı

Prefix `/api/v1`.

| Alan | Endpoint'ler |
| --- | --- |
| Journey | `POST /journeys`, `GET /journeys`, `GET /journeys/{id}`, `PUT /journeys/{id}/draft`, `POST /journeys/{id}/validate`, `POST /journeys/{id}/publish`, `POST /journeys/{id}/pause`, `POST /journeys/{id}/resume`, `GET /journeys/{id}/versions` |
| AI | `POST /ai/journeys/generate`, `POST /ai/journeys/modify`, `POST /ai/messages/generate` |
| Customer/Event | `POST /customers`, `GET /customers`, `GET /customers/{id}`, `POST /events`, `GET /customers/{id}/events` |
| Test | `POST /test-scenarios`, `POST /test-runs`, `POST /test-runs/{id}/advance`, `GET /test-runs/{id}`, `GET /test-inbox`, `GET /test-messages/{id}`, `POST /messages/test-send` |
| Monitor | `GET /journeys/{id}/executions`, `GET /executions/{id}`, `GET /journeys/{id}/stats`, `GET /journeys/{id}/analytics` |

Aksiyon API'lerinde auth, validation, idempotency. OpenAPI otomatik üretilebilir.

## 17. Teknik kalite ve güvenlik

Backend: context propagation, structured errors, bounded retry, cancellation, graceful shutdown, transactions, idempotency, query/index optimization, OpenTelemetry, health/readiness.

Frontend: typed client, loading/error/empty, responsive admin, accessible forms, autosave feedback, unsaved changes, keyboard navigation, toast, error boundaries, skeleton.

Güvenlik: authentication, Admin/Editor/Viewer, workspace isolation, güvenli API key, environment secrets, PII masking, consent, audit, HTML sanitization, schema validation, API rate limit, test allowlist, production/test isolation. Retention/deletion politikası dokümante edilir.

## 18. Geliştirme fazları

Sıra zorunludur. Her faz doğrulanabilir çıktı üretir; büyük fazlar tutarlı alt commit'lere ayrılır.

### Faz 1 — Repository & Foundation

Monorepo; Vue frontend; Go backend; PostgreSQL; Docker Compose; migrations; Makefile; lint/format/test; GitHub Actions; Conventional Commits kontrolü.

**Kabul:** Temiz ortamda kurulabilir; frontend/backend/health çalışır.

Örnek commit'ler: `chore(repo): initialize customer journey composer monorepo`, `build(dev): add docker compose development stack`, `ci: add lint typecheck and test workflows`, `docs: add architecture and local setup guide`.

### Faz 2 — Authentication & Journey Management

Login/logout, workspace, journey CRUD/list, draft persistence, temel panel.

**Kabul:** Kullanıcı giriş yapıp journey oluşturur ve yeniden açar.

Örnekler: `feat(auth): implement workspace authentication`, `feat(journeys): add journey management endpoints`, `feat(ui): build journey dashboard and listing`, `test(journeys): cover draft persistence and permissions`.

### Faz 3 — Visual Canvas & Validation

Vue Flow, node library/config panels, connections, undo/redo, autosave, graph validation, versioning.

**Kabul:** Elle geçerli akış kaydedilir; hatalı akış publish edilemez.

Örnekler: `feat(builder): implement visual journey canvas`, `feat(builder): add node configuration panels`, `feat(validation): add workflow graph validator`, `feat(versions): persist immutable workflow definitions`, `test(builder): cover graph validation rules`.

### Faz 4 — Customer & Event Infrastructure

Profiles, attributes, ingestion, deduplication, timeline, entry matching.

**Kabul:** Gerçek API event'i müşteri için execution başlatır.

Örnekler: `feat(customers): implement customer profiles`, `feat(events): add validated event ingestion`, `feat(events): enforce event idempotency`, `test(events): cover duplicate and concurrent ingestion`.

### Faz 5 — Persistent Runtime

State machine, steps, conditions, durable timers, Wait for Event, global exit, claim/lease, crash recovery.

**Kabul:** Restart ve event/timeout yarışları tutarlı.

Örnekler: `feat(runtime): implement persistent journey executions`, `feat(runtime): add condition and wait processing`, `feat(runtime): add event waiters and global exit rules`, `fix(runtime): guard concurrent state transitions`, `test(runtime): cover crash recovery and timeout races`.

### Faz 6 — Email & SMS

Editor, personalization, Mailpit, mock SMS, delivery jobs, retry/backoff, consent/cap.

**Kabul:** Node doğru mesajı provider adapter'a iletir; durum izlenir.

Örnekler: `feat(messaging): add email and sms composition`, `feat(delivery): implement durable delivery jobs`, `feat(delivery): integrate email and mock sms providers`, `feat(policy): enforce consent and frequency caps`, `test(delivery): cover retries and idempotent scheduling`.

### Faz 7 — AI Journey Composer

Claude **ve OpenAI**, prompt-to-workflow, message generation, structured output, graph validation, diff/onay. Canlı LLM testleri ertelenir.

**Kabul:** Prompt geçerli, düzenlenebilir journey taslağı üretir; insan onayı zorunlu.

Örnekler: `feat(ai): generate journeys from natural language`, `feat(ai): add structured workflow validation`, `feat(ai): support journey modifications with diff`, `test(ai): cover invalid and incomplete model outputs`.

### Faz 8 — Preview & Test Lab

Message/customer/path preview; virtual clock; scenario runner; fault injection; isolation.

**Kabul:** Gerçek zaman beklemeden birden fazla event ile test.

Örnekler: `feat(preview): add personalized message previews`, `feat(testing): implement isolated journey simulator`, `feat(testing): add virtual clock and scenario replay`, `test(testing): verify sandbox isolation`.

### Faz 9 — Test Inbox

Recipients, persistence, inbox, detay/render/trace, test send.

**Kabul:** Tüm test mesajları müşteri ve node ile eşleşir.

Örnekler: `feat(test-inbox): persist simulated message outputs`, `feat(test-inbox): build recipient and message views`, `feat(test-inbox): expose decision and delivery traces`, `test(test-inbox): cover preview and message isolation`.

### Faz 10 — Live Monitor & Analytics

Node stats, customer tracking, inspector, funnel, conversion, channels.

**Kabul:** Gerçek execution verisi canvas/raporlara doğru yansır.

Örnekler: `feat(monitor): add live journey execution tracking`, `feat(monitor): visualize customer paths on canvas`, `feat(analytics): add journey and channel metrics`, `test(analytics): verify counts against execution records`.

### Faz 11 — Lifecycle & Reliability

Publish/pause/resume/sunset; version isolation; failed recovery; audit; security; concurrency; observability; CI security.

**Kabul:** Lifecycle, provider hataları, eşzamanlı event, worker restart güvenli.

Örnekler: `feat(lifecycle): implement controlled journey activation`, `fix(runtime): prevent actions after journey exit`, `feat(observability): add execution tracing and metrics`, `test(e2e): cover journey lifecycle and concurrency`, `ci: add dependency and container security scanning`.

### Faz 12 — Final Product & Demo

Templates, demo customers/events, E2E, benchmark, README, architecture, screenshot/video, deployment doğrulaması.

**Kabul:** Temiz kurulumda AI generation → test → publish → execute → analytics.

Örnekler: `feat(templates): add onboarding and cart recovery journeys`, `test(e2e): cover complete customer journey lifecycle`, `docs: add benchmark results and architecture diagrams`, `chore(release): prepare initial mvp release`.

## 19. Conventional Commits & Git protokolü

Format: `<type>(<scope>): <short description>`; scope gerektiğinde. Türler: feat, fix, test, refactor, perf, docs, ci, build, chore. İngilizce kısa/anlamlı mesaj. `wip`, `update`, `changes`, `final fix` yok.

Her bağımsız çalışan dilimden sonra commit. Hedef 30–90 dakika veya tamamlanmış alt özellik; süre doldu diye kırık kod commit edilmez. Büyük faz tek commit olmaz; model/migration/API/UI/test tutarlı parçalar. Alakasız değişiklikler birleşmez. Secret, `.env`, test credential, PII commit edilmez.

Commit öncesi, komutlar oluşturulduktan sonra:

```sh
make fmt-check
make lint
make typecheck
make test
git diff --check
git status --short
```

Faz sonunda integration ve gerekli E2E, çalışma ağacı kontrolü. `main` korunarak kısa ömürlü feature branch kullanılabilir. Dilim raporu: özellik, önemli modüller, testler/sonuçlar, gerçek commit hash, sonraki iş. Oluşturulmamış commit veya çalıştırılmamış test için başarı iddia edilmez.

## 20. Test ve performans kabul kriterleri

- Prompt → geçerli journey; canvas düzenleme.
- Event → gerçek execution; durable delay/event wait; doğru condition.
- Email/SMS provider adapter; customer preview; virtual clock; inbox.
- Test production değiştirmez; Track Customer; tutarlı analytics.
- Duplicate event logical action üretmez; restart görev kaybettirmez.
- Publish/pause/sunset testleri; version isolation; temiz CI.

En az **10.000 aktif execution** yük senaryosu: ingestion throughput, event-to-execution latency, due-job latency, worker throughput, pool pressure, retry, failed/duplicate actions, memory, p50/p95/p99. Yalnızca gerçek benchmark çıktıları raporlanır.

## 21. MVP dışı

WhatsApp; web/native push; gerçek zamanlı CDP; A/B/experimentation; next best channel; AI decisioning/contextual bandits; runtime AI reasoning node; advanced segmentation; complex loops; tenant billing; büyük Kubernetes; Figma; send-time optimization. Çekirdek runtime ve testing sonrası ayrı fazlar.

## 22. Son ürün demosu

1. Create Journey: doğal dille abandoned cart.
2. AI Generation: nodes/edges.
3. Visual Edit: condition ve mesaj.
4. Preview: customer email/SMS.
5. Test: cart event + virtual clock.
6. Inbox: mesaj ve karar izi.
7. Publish: insan onayı.
8. Live Execution: demo store/event generator.
9. Track Customer: canvas path.
10. Analytics: entry/conversion/skip/send.
11. Reliability: restart/duplicate event.

**Customer Journey Composer is an AI-powered customer journey orchestration platform that turns natural-language instructions into executable, testable, and observable customer workflows.**

Temel farklılaştırıcı: AI görsel workflow, gerçek persistent execution ve deterministic testing aynı üründedir.
