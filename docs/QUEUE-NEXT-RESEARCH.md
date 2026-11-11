# Kalan kuyruk maliyeti: araştırma ve sonraki deneyler

11 Kasım 2026. Bu belge uygulama değişikliği değildir. [Son ölçümleri](QUEUE-FOLLOWUP.md), mevcut sorguları ve PostgreSQL/Camunda/River kaynaklarını karşılaştırır. Redis veya yeni bir workflow motoruna geçiş için mevcut veride gerekçe yok.

## Yeni bulgu: analytics kuyruğunda toplu silme planı

`analytics/projector.go` her turda 1.000 satır seçiyor; ardından `DELETE ... USING claimed` ile siliyor. İzole PostgreSQL üzerinde mevcut şema ve primary key ile 100.000 sentetik satırlık bir plan deneyi yapıldı. Mevcut plan, 1.000 satırı seçtikten sonra silme tarafında **100.000 satırlık sequential scan** yapıyor. Seçilen ID'leri `id = ANY(ARRAY(SELECT id FROM claimed))` ile hedefleyen alternatif, aynı silme tarafında **1.000 satırlık primary-key index scan** kullanıyor.

[Ham deney, SQL, ortam ve EXPLAIN planları](benchmarks/projector-delete-probe.json) altışar tekrar içeriyor; sıra dönüşümlü değiştirildi. Her sorgu BEGIN/ROLLBACK içinde çalıştı; sonunda 100.000 satırın tamamı duruyor. Üretici, eşzamanlı kilitler, aggregation ve uygulama worker'ları yok. Bu sıcak bir mikro deneydir; production throughput veya journey hızlanması değildir. Planın başka veri dağılımlarında korunacağı ayrıca doğrulanmalıdır. [PostgreSQL EXPLAIN](https://www.postgresql.org/docs/17/sql-explain.html), gerçek satır/loop/buffer bilgileriyle bu ayrımı incelemeyi sağlar.

Bu mikro deneyde median sorgu süresi mevcut planda **18,722 ms**, hedefli alternatifte **6,376 ms**. Bu sayı toplam 65–67 saniyelik gönderim süresinin üç kat hızlanacağı anlamına gelmez; yalnızca belirli query planı için bir aday kanıtıdır.

**İlk öneri:** Mevcut bounded claim ve atomik delete+projection transaction'ını koruyarak yalnızca seçilmiş satırları hedefleyen silme planını denemek. Mevcut primary key yeterli görünüyor; yeni indeks eklemek bu deneyin gereği değil. Queue deletion ile summary güncellemesi aynı commit'te kalmalı. [River transactional completion](https://riverqueue.com/docs/transactional-job-completion), iş tamamlaması ile ilişkili veritabanı değişikliğini aynı transaction'da tutan benzer güvenilirlik desenini belgeler.

## Uygulamadaki ölçülmüş maliyet

Go 1.26.10 ile eşleştirilmiş `readbatch-1/2` sonuçları:

| İş                                       |       Çağrı | Toplam SQL execution süresi |
| ---------------------------------------- | ----------: | --------------------------: |
| Scheduler continuation seçimi            |       9.688 |                15,1–19,8 sn |
| Projector queue claim/delete/JSON dönüşü |     125–131 |                 8,1–11,0 sn |
| Runtime due-job claim                    | 7.815–7.926 |                  4,2–4,5 sn |
| Delivery claim seçimi                    |      11.632 |                  6,4–7,9 sn |

Bunlar eşzamanlı sorguların kümülatif süreleridir; toplam journey duvar saatiyle toplanamaz ve tamamı kilit beklemesi sayılamaz. Worker pool tepe kullanımı 14/20; toplam acquisition süresi 162–233 ms. Bu veriler pool'u büyütmenin ilk çözüm olduğuna işaret etmiyor. Go profili client/API/worker'ı birlikte kapsıyor; PostgreSQL CPU'sunu ölçmüyor.

Benchmark statement istatistikleri, son mesajın provider tarafından kabulünden sonraki `metric_queue` drain'ini de kapsar. Bu yüzden projector'ın 8–11 saniyesinin tamamı gönderim kritik yolunda değildir. Paylaşılan PostgreSQL kaynaklarına etkisi ve toplam kabul süresindeki değişim, analytics freshness korunarak ayrıca ölçülmelidir.

## İkinci öneri: toplu aday bulma, tek müşteri transaction'ı

Scheduler şu an her mesaj için execution/customer/settings/lifecycle join'ini yeniden kuruyor. Küçük bir aday ID sayfası tek sorguyla bulunup, her ID kendi kısa transaction'ında PK üzerinden tekrar okunabilir. Aday listesi yetki veya sahiplik sağlamaz: customer kilidi, yeni snapshot ile consent/cap/lifecycle kontrolü, outbox kilidi ve unique logical action korunur. Kilitli veya geçersiz aday sonraki tura bırakılır; restart sonrası liste yeniden veritabanından oluşur.

Bu henüz ölçülmemiş bir tasarım hipotezidir. Global queue discovery maliyetini azaltabilir; per-ID okuma ve commit maliyetini ortadan kaldırmaz. Yeni lease/sahiplik modeli eklemek ilk deney için gerekli olmayabilir. [River metrics](https://riverqueue.com/docs/metrics) toplu iş alma süresi ve alınan iş sayısını ayrı izler; bizim deneyde de discovery, kilit beklemesi, transaction ve boş aday oranı ayrı ölçülmeli.

Önemli fairness riski: yalnızca ilk 32 ID'yi kilitsiz almak, bu müşterilerin hepsi kilitliyken sonraki uygun işleri aç bırakabilir. Mevcut sorgunun limit öncesi paused/locked atlama davranışı korunmalı; bounded devam sayfası ve daha sonra yeniden ziyaret mekanizması bu durumu test etmeden aday önbelleği uygulanmamalı. Büyük bir müşteri grubunu tek transaction'da kilitlemek çözüm değildir.

## Üçüncü öneri: kapasite eşiğine göre runtime refill

Mevcut worker, boşalan slotları 2 ms boyunca biriktirip yeniden claim ediyor. Normal timer burst'ünde hâlâ yaklaşık 7.900 claim çağrısı var. [Camunda 8.9 Java worker](https://docs.camunda.io/docs/8.9/apis-tools/java-client/job-worker/) bekleyen iş sayısı yapılandırılmış kapasitenin yaklaşık %30'una düşünce yeni işler alır. Bu oran evrensel ayar değildir.

Bizde eşik + kısa maksimum bekleme birlikte denenebilir: tüm slotlar boşken bekleme yok, kapasite aşımı yok, yavaş bir müşteri diğer slotları durdurmuyor. İşlenemeyecek kadar job önceden claim edilmez; lease kuyrukta tüketilmez. Timer ve delivery için ağırlıklı kapasite, ancak starvation görülürse ayrı deney olur. [River multiple queues](https://riverqueue.com/docs/multiple-queues), pahalı işler ile gecikmeye duyarlı işler için worker kapasitesi ayırmayı açıklar; aynı PostgreSQL'i paylaşmak CPU/I/O izolasyonu sağlamaz.

## Ölçüm boşlukları ve sonraya bırakılacaklar

- **Gerçek prepared planlar:** Şu an `total_plan_time` toplanmıyor. İzole teşhiste `EXPLAIN EXECUTE` ile custom/generic planlar karşılaştırılmalı; global `force_custom_plan` uygulanmamalı. PostgreSQL [PREPARE](https://www.postgresql.org/docs/17/sql-prepare.html) plan seçimini, [pg_stat_statements](https://www.postgresql.org/docs/17/pgstatstatements.html) planning ölçümünü ve açılmasının ek maliyetini açıklar. Enstrümantasyonlu baseline ile aday eşleştirilmeli.
- **MVCC ve vacuum:** Queue tablolarındaki dead tuple/index sayfaları ve autovacuum geçmişi ölçülmeli. [Routine vacuuming](https://www.postgresql.org/docs/17/routine-vacuuming.html), update/delete sonrası eski satırların hemen kalkmadığını ve tablo bazlı ayarları açıklar. Bloat henüz kanıtlanmadı; rastgele VACUUM FULL veya indeks eklemek önerilmiyor.
- **Projector taşıması:** Şu an 1.000 satırlık JSON Go'ya alınıp üç ayrı SQL adımına tekrar gönderiliyor. Bounded SQL-side aggregation adaydır; ilk adımda silme planıyla birlikte değiştirilmemeli. Atomiklik, unique-customer paydası ve watermark korunmalı.
- **LISTEN/NOTIFY:** [PostgreSQL NOTIFY](https://www.postgresql.org/docs/17/sql-notify.html) bir uyandırma sinyalidir; kalıcı iş kaydı değildir. Polling yedeği gerekir. Boş/düşük trafikte yararlıdır; hazır 10.000 işin işlem maliyetini tek başına çözmez.

## Kabul kapısı

Sıra: **projector delete planı → scheduler aday sayfası → runtime refill**. Her değişiklik tek başına, Go 1.26.11 üzerinde yeni baseline ve en az iki aday koşusuyla karşılaştırılmalı. Eski Go sonuçları yeni baseline gibi etiketlenmemeli. Aynı anda dolan 10.000 timer, 60 saniyeye yayılan deadline, sıcak müşteri, kilitli/paused işler ve yavaş provider senaryoları korunmalı. Sıfır duplicate/failed logical action, doğru analytics toplamları ve güvenli restart ön koşuldur. SQL süresi düşerken toplam gönderim veya timer p99 kötüleşirse değişiklik kabul edilmez.
