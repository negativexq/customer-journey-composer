# UI/UX incelemesi ve uygulanabilir iş listesi

İnceleme: 10 Kasım 2026. Kapsam: Customer.io, Braze Canvas, Iterable, Knock ve Loops'un resmi dokümanları ve herkese açık ürün ekranları. Ücretli hesaplarda görev tamamlama/kullanılabilirlik testi yapılmadı. Aşağıdaki öneriler bizim kod incelememizden çıkarılmıştır; rakiplerde belgelenmeyen özelliklerin bulunmadığı anlamına gelmez.

## Rakiplerde görülen yaklaşımlar

| Ürün / kaynak                                                                                                                                                   | Doğrulanan yaklaşım                                                                                                                                   | Bizim ürüne karşılığı                                                                                                      |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| [Customer.io builder](https://docs.customer.io/messaging/send/workflows/builder/) ve [yayına hazırlık](https://docs.customer.io/get-started/quick-start-guide/) | Geniş canvas, bağlama göre sağ panel, yayın öncesinde eksikleri ve ayarları birlikte inceleme.                                                        | Builder'ın üst alanını sadeleştir; yayın kontrollerini hata cümlesi yerine düzeltme bağlantılı bir panelde göster.         |
| [Braze: Preview user paths](https://www.braze.com/docs/user_guide/messaging/canvas/testing_canvases/preview_user_paths)                                         | Test müşterisi seçimi; zamanlama, mesaj ve adım koşullarının test sonucunda incelenmesi.                                                              | Mevcut virtual clock'u koru; senaryo girdisi, izlenen yol ve mesaj sonucunu daha görünür bağla.                            |
| [Iterable: Journey Engagement](https://support.iterable.com/hc/en-us/articles/10044063369492-Monitoring-Journey-Engagement)                                     | Node üzerinde en fazla dört seçili metrik; node'dan kampanya analizine, journey'den toplu analize geçiş.                                              | Canvas'ta az sayıda anlamlı sayı; ayrıntı için filtreli Analytics/Execution Inspector bağlantısı.                          |
| [Knock: Debugging workflows](https://docs.knock.app/send-notifications/debugging-workflows)                                                                     | Run'a API kaydı, müşteri veya workflow'dan erişim; mesajın oluşmama nedeni; olay detayında zaman çizelgesi. Activity incelenirken akışı durdurabilme. | Event → execution → karar → mesaj arasında bağlamı koruyan gezinme; okunabilir karar kartları ve sabit kalan detay paneli. |
| [Loops: Workflows](https://loops.so/docs/workflows)                                                                                                             | Bağlantı üstünde “+” ile adım ekleme; kapanabilir paneller; seçili node'a göre inspector, seçim yokken validation.                                    | Node eklemek için sürekli kütüphaneye dönmeyi azalt; canvas alanını panel kontrolüyle kullanıcıya bırak.                   |
| [Customer.io: Metrics](https://docs.customer.io/messaging/metrics/automation-metrics/)                                                                          | Journey ve mesaj ölçümlerinin tarih/kohort anlamı açıkça ayrılır.                                                                                     | Filtreyi görsel olarak büyütmekten sonra asıl iş, hangi kartın hangi tarih ve paydayı kullandığını anlaşılır göstermek.    |

Customer.io builder, Loops workflow ve Knock event-inspector görselleri görsel olarak da incelendi. Görseller ürün arayüzüne kopyalanmadı; kaynak bağlantıları referans olarak tutuldu.

## 10 Kasım incelemesindeki eksikler ve öncelik

### P0 — Önce anlaşılabilirlik ve güvenli işlem

1. **Yayın kontrol paneli.** `JourneyEditor.vue` şu anda `/publish-check` sonuçlarını tek hata metninde topluyor, ardından tarayıcı `confirm` açıyor. `PublishReviewPanel` ile graph, kanal/provider, consent/cap, global exit ve test kanıtlarını gruplandır. Her engeli ilgili node/ayar/teste bağla. Sunucu kontrolü ve revision guard otorite olarak kalır. Kabul: başarısız kontrol paneli kapatmaz, düzeltme ilgili yere gider; Viewer yayınlayamaz; son onay dışında yayın POST'u yoktur.
2. **Draft / live sürüm ayrımı.** Aynı editörde journey durumu ve düzenlenen taslağın anlamı açık olmalı: “Editing draft · Live v3”. Mevcut müşterilerin başladıkları sürümde devam edeceğini yayın panelinde belirt. Kabul: taslak değişikliği canlı akış değişmiş gibi gösterilmez; Versions karşılaştırmasına tek geçiş vardır.
3. **Execution timeline ve karar kartı.** `LiveMonitor.vue` geçişleri, koşulları ve bekleyen işleri ağırlıkla JSON gösteriyor; `TestInbox.vue` Decision/Trace görünümü de aynı sorunlu. `ExecutionTimeline`, `DecisionCard`, `AttemptList`: zaman, koşul sonucu, consent/cap nedeni, beklenen event ve deadline okunabilir alanlar olmalı. JSON gelişmiş ayrıntı olarak kalabilir. Kabul: “SMS neden gönderilmedi?” sorusu JSON okumadan yanıtlanır; üç provider denemesi tek logical send altında gösterilir.
4. **Geçici erişim hatası ile oturumu ayır.** `App.vue` ilk `/auth/me` isteğinin tüm hatalarında giriş ekranını gösteriyor. 401 anonim akışına; 503/ağ hatası ise URL ve oturumu koruyan tekrar deneme görünümüne gitmeli. Kabul: geçici 503 sonrası yeniden giriş gerekmez; derin bağlantı korunur. API artık DB kaynaklı auth hatasını yanlış 401 yerine 503 döndürüyor; frontend karşılığı henüz yapılmadı.

### P1 — Tasarım ile test ve ölçümü birbirine bağla

5. **Test çalışma alanı.** `TestLab.vue`: müşteri / event dizisi / sanal saat / sonuç bölgeleri. Ortak `CustomerProfileFields` kullan; JSON'u ileri düzey bölümde tut. Node sonucu mesaj varsa doğrudan filtreli Test Inbox'a gider. “Simulated” ve gerçek “Send test” eylemleri ayrı görünür. Kabul: cart → 2h → email → purchase → exit senaryosu aynı bağlamda izlenir; production profili/provider'ı değişmez.
6. **Analytics kapsamını açık tut.** Mevcut geniş `FilterSelect` ve `PeriodSelect` korunur. All-time kartlar ile dönem grafiği zaten ayrı etiketli; yeni çalışma bu anlamı bozmaz. Tarih, journey/version ve veri tazeliği aynı toolbar'da, kısa okunabilir metinle gösterilir. Canvas'ta önce entered/waiting/completed/failed; kanal için accepted/skipped/failed kullanılır. Open/click verisi yoksa uydurulmaz; accepted “delivered” olarak adlandırılmaz.
7. **Bağlantıdan adım ekleme.** `WorkflowCanvas.vue`: edge üzerindeki klavye ile erişilebilir “+”, tür seçimi ve portu koruyarak araya ekleme. Aynı işlem undo ile tek adımda geri alınır. Dokunmatik kullanımda hover gerekmez. İki uçlu condition/wait-event portları otomatik yanlış dala bağlanmaz.
8. **Detay ekranları arası tutarlılık.** Customer, event, execution ve message linkleri URL'de seçimi korumalı. Ortak `DetailDrawer` ve durum badge'leri kullanılmalı. Tabloyu otomatik yenilemek seçili satırı kaydırmamalı; “yeni kayıtlar” bildirimi ve elle yenileme kontrolü bulunmalı.

## Component ve performans kuralları

- Kompakt sol menü korunur: 16px ikon, 13px yazı, 34px satır. Sistem fontu, nötr yüzeyler ve sınırlı vurgu rengi korunur. Yeni dekoratif font, gradient kart veya büyütülmüş menü gerekmez.
- Mevcut `FilterSelect`, `PeriodSelect`, `MetricCard`, `StatusBadge` ortak kaynak olarak kullanılmalı; her ekrana ayrı dropdown yazılmamalı. Açık seçenek alanı metni daraltmamalı; klavye/focus/escape ve küçük ekran kontrolleri kabul kriteridir.
- Inspector ayrıntıları yalnızca seçimde yüklenir. Canvas node başına ayrı API isteği yapılmaz. Metrikler mevcut toplu summary API'lerinden alınır; görüntü uğruna ham execution taraması eklenmez.
- Otomatik polling görünmeyen sekmede durur; eşzamanlı istekler birleşir; obsolete yanıtlar seçili execution'ı değiştiremez. Telemetry UI tasarımı için yeni bir production sorgu zinciri oluşturmaz.
- Publish paneli açılınca bir bounded kontrol isteği; gezinme için mevcut node/issue kimlikleri kullanılır. Güvenlik kontrolleri istemci kararına bırakılmaz.

## Önerilen uygulama sırası

Önce geçici auth hatası görünümü ve draft/live ayrımı; sonra yayın kontrol paneli; ardından ortak timeline/decision bileşenleri. Bunlar oturduktan sonra Test Lab bağlantıları, Analytics drill-down ve edge üzerinden ekleme uygulanmalı. Her dilim masaüstü/mobil, klavye ve gerçek API hata senaryosuyla doğrulanmalı.

Bu belge araştırmanın tarihli bulgularını korur; güncel uygulama durumu aşağıdadır. A/B, yeni kanallar veya journey içinde AI reasoning eklemek MVP kapsamına alınmadı.

## 11 Kasım uygulama durumu

- P0/1–3: Yayın kontrol/onay paneli, draft/live ayrımı, okunabilir execution timeline, karar ve attempt kartları uygulandı. Test Lab run-scoped Test Inbox ile aynı bağlamda çalışıyor.
- P0/4: İlk oturum kontrolünde 401, geçici 503 ve ağ hatası ayrıldı. Geçici hatalar URL'yi koruyan tekrar deneme ekranı gösteriyor. Journey detay isteğinin geçici hatası da listeye yönlendirmiyor.
- P1/7: Bağlantı üstünden adım ekleme, açık yeni dal seçimi ve tek undo uygulandı. Trigger/Exit araya ekleme seçenekleri arasında bulunmuyor; mevcut bağlantı korunuyor.
- P1/8: Ortak activity drawer; customer → event → execution → message bağlantıları ve seçimin URL üzerinden yenileme/geri/ileri ile korunması uygulandı. Live Monitor execution/delivery seçimi de URL'de korunur. Lazy detail sorguları, sıralama ve workspace sınırları korunur. Listeyi elle yenilemek açık drawer'ı değiştirmez.
- Analytics'te kapsam/denominatör etiketleri ve branched node reach gösterimi mevcut; her node'dan özel filtreli Analytics drill-down ayrı bir geliştirme olarak kalır. Otomatik yeni-kayıt bildirimleri bu teslimatta yoktur; customer/event listeleri elle yenilenir.
- Doğrulama: `make check`, tüm paketlerde PostgreSQL entegrasyonları ve 44/44 masaüstü/mobil tarayıcı testi. Ücretli LLM çağrısı veya harici deployment yapılmadı.
