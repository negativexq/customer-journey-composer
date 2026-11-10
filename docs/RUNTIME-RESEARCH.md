# Runtime darboğazları: benzer sistemlerde çözümler

İnceleme tarihi: 10 Kasım 2026. Bu belge resmi doküman/kaynak kod araştırmasıdır; aşağıdaki öneriler henüz runtime'a uygulanmadı. Bizim ölçümlerimiz [operasyonel performans raporunda](OPERATIONS-PERFORMANCE.md). Rakip throughput rakamlarını kendi ürünümüz için sonuç kabul etmiyoruz.

## 1. Kapasite eşiğine göre toplu iş alma

**Doğrulanan örnek:** [Camunda 8.9 Java worker](https://docs.camunda.io/docs/8.9/apis-tools/java-client/job-worker/) kalan iş sayısı `maxJobsActive` değerinin yaklaşık %30'una düşünce yeniden kapasitesi kadar iş alır. Böylece her biten iş için ayrı polling yapılmaz ve tüm grubun tamamlanması beklenmez. Bu oran ilgili istemci sürümünün davranışıdır; evrensel ayar değildir.

**Bizim karşılığı:** Önceki grup bariyerine dönmeden, boş slot eşiği ve en fazla bekleme süresi birlikte kullanılabilir. Mevcut 2ms coalescing yalnızca zamana bağlıdır. Başlangıçta küçük grupları doldurma, eşik altına düşünce refill ve düşük trafikte maksimum refill gecikmesi ayrı kontrol edilir. Tüm slotlar boşken yeniden dolum geciktirilmez. İşlenemeyecek kadar job önceden claim edilmez; lease süresi iç kuyrukta tüketilmez. Eşik/timeout değerleri ölçümle seçilmeli.

**Beklenen etki:** Claim sayısı ve DB round trip azalır. Çok agresif batching düşük trafikte gecikme yaratabileceğinden timer p95/p99 ve boş kalan worker kapasitesi birlikte ölçülmeli. Bu bir hipotezdir; kazanç henüz ölçülmedi.

## 2. Her görsel node'u ayrı scheduler işi yapmamak

**Doğrulanan örnek:** [Camunda 7 transaction modeli](https://docs.camunda.org/manual/7.24/user-guide/process-engine/transactions-in-processes/) bir bekleme noktasından diğerine kadar ilerlemeyi tek transaction'da yapabilir; async continuation ile ek sınırlar tanımlanır. Camunda 7 eski ürün hattıdır; burada önerilen yeni bağımlılık değil, belgelenmiş yürütme desenidir. [Temporal Local Activities](https://docs.temporal.io/design-patterns) da kısa, idempotent işleri worker içinde çalıştırarak server round trip azaltan bir başka örnektir; kalıcılık modeli bizim PostgreSQL transaction modelimizle aynı değildir.

**Bizde doğrulanan durum:** `runtime/worker.go` içindeki `scheduleNext` her sonraki node için `scheduled_jobs` kaydı üretir. Sonraki node tekrar claim edilir, müşteri/execution/job okunup kilitlenir ve workflow tekrar ayrıştırılır. Condition gibi kısa bir işlem de bu maliyeti taşır.

**Öneri:** Tek execution için kısa, deterministik adımları sınırlı bir işlem turunda ilerlet. Örneğin timer resume → condition → mesaj outbox niyeti → sonraki wait. Her node'un step geçmişi korunur; yalnızca scheduler'a yeniden dönme sayısı azaltılır. Yeni wait/event beklemesi, harici işlem sınırı veya adım/zaman bütçesi dolunca kalıcılaştırıp bırakılır. Sınırsız “run to completion” uygulanmaz.

**Korunacaklar:** Customer/journey/execution kilit sırası, global exit/event-timeout tek kazananı, pause sınırı, immutable version, node başına tek logical action, atomik outbox ve retry. Aynı turdaki attribute değişikliği sonraki condition'a görünmeli; virtual clock ile semantik uyum test edilmeli. Dış provider çağrısı bu transaction'a alınmaz. Daha uzun bir transaction'ın müşteri kilidini uzatabileceği özellikle ölçülür; kazanç otomatik kabul edilmez.

## 3. Timer ve pahalı gönderim işleri için kapasite ayırmak

**Doğrulanan örnek:** [River multiple queues](https://riverqueue.com/docs/multiple-queues) her kuyruğa ayrı worker kapasitesi verir; pahalı işlerin diğer işleri aç bırakmasını önlemek ve öncelikli işlere kapasite bırakmak resmi kullanım örnekleridir. [Temporal server](https://docs.temporal.io/temporal-service/temporal-server) kalıcı timer/history işlerini ve task dispatch'i ayrı bileşenlerde yönetir; History Shard'lar timer kuyrukları taşır.

**Bizim karşılığı:** Runtime ve delivery loop'ları ayrı, ancak aynı worker pool/veritabanını paylaşır. Yalnızca yeni goroutine açmak kaynak izolasyonu sağlamaz. Timer/resume, normal geçiş ve outbox/delivery için ölçülmüş kapasite bütçeleri veya ağırlıklı seçim denenebilir. Toplam bağlantı/worker sayısını başta artırmadan karşılaştırılmalı. Kesin öncelik verip delivery'yi aç bırakmak yerine fairness ve maksimum bekleme izlenmeli. Ayrı DB pool bağlantı bütçesini ayırabilir; aynı PostgreSQL CPU/I/O ve satır kilitleri için gerçek izolasyon sağlamaz.

**Sınır:** Temporal'ın sharding mimarisi, bizim 10.000-execution testimiz için hemen sharding gerektiği kanıtı değildir. Kafka/Redis veya motor değişimi bu araştırmanın sonucu olarak zorunlu görülmüyor.

## 4. Bildirimle uyandırma, polling ile güvenli yedek

**Doğrulanan örnek:** [River Go client kaynağı](https://github.com/riverqueue/river/blob/master/client.go) `FetchCooldown` ile minimum fetch aralığını, `FetchPollInterval` ile periyodik yedeklemeyi ve LISTEN/NOTIFY üzerinden hızlı uyandırmayı ayırır. [Camunda job workers](https://docs.camunda.io/docs/components/concepts/job-workers/) boş polling maliyetine karşı long polling ve streaming anlatır.

**Bizim karşılığı:** PostgreSQL tablosu otorite olarak kalırken commit sonrası uyandırma sinyali + bir sonraki deadline'a göre bekleme + periyodik yeniden tarama düşünülebilir. Bildirim işin kendisi değildir; kaçırılan bildirim veri kaybına dönüşmemeli. NOTIFY sayısı iş başına sınırsız büyütülmemeli.

**Öncelik:** Bu çoğunlukla boş/düşük trafikte polling maliyetini ve başlangıç gecikmesini azaltır. 10.000 hazır timer'ın işlenmesi sırasındaki node/transaction maliyetini tek başına çözmez; ilk optimizasyon olmamalı.

## 5. Toplu completion yazımı her yerde aynı şekilde güvenli değil

**Doğrulanan örnek:** [River BatchCompleter kaynağı](https://github.com/riverqueue/river/blob/master/internal/jobcompleter/job_completer.go) başarılı tamamlamaları biriktirip toplu günceller. [River transactional completion](https://riverqueue.com/docs/transactional-job-completion) ise uygulama değişikliği ve job tamamlamanın aynı transaction'da yapılmasını destekler.

**Bizim karşılığı:** PostgreSQL batch/çok satırlı yazım round trip azaltmak için adaydır. Ancak execution state, step sonucu, job completion ve sonraki iş/outbox'ın atomik olması gerekir. Bunlardan yalnızca job sonucunu RAM'e alıp daha sonra yazmak mevcut crash/race güvenliğini değiştirebilir. Önce aynı transaction içindeki SQL akışını azaltmak incelenmeli; farklı müşterileri tek büyük transaction'da toplamak ilk tercih olmamalı.

## 6. Kilitleri kaldırmadan önce beklemenin kaynağını ölçmek

[PostgreSQL locking dokümanı](https://www.postgresql.org/docs/17/explicit-locking.html) tutarlı kilit sırası ve kısa transaction'ların önemini açıklar. [İstatistik dokümanı](https://www.postgresql.org/docs/17/monitoring-stats.html) `pg_stat_activity` içindeki wait event alanlarıyla mevcut beklemeyi ayırmayı sağlar.

Bizde `SELECT ... customers ... FOR UPDATE` toplam süresinin 1,9s → 48,9s yükselmesi, 48,9 saniyenin tamamının kilit beklemesi olduğunu kanıtlamaz. `pg_stat_statements` sorgu süresi tek başına CPU, I/O ve kilit payını ayırmaz. Bunu önceki değerlendirmelerdeki “kilit sorgusu pahalandı” ifadesinden daha kesin bir teşhis olarak sunmamak gerekir.

`delivery/service.go` zaten `DISPATCHING` durumunu commit ettikten sonra provider'a gider ve sonucu ayrı transaction'da yazar. Ağ çağrısı boyunca müşteri kilidi tutmuyoruz; bu yönde gereksiz yeniden tasarım yapılmamalı. Aynı müşteri için consent, frequency cap, exit ve attribute değişikliği birbiriyle tutarlı kalmak zorunda; `FOR UPDATE` kontrolleri topluca kaldırılamaz.

## Bizim için uygulama sırası ve karar kapıları

1. **Önce teşhis:** İzole benchmark'ta bounded `pg_stat_activity` wait-event/blocker örnekleme, transaction süreleri, claim başına alınan iş sayısı, boş claim oranı, node başına SQL sayısı ve Go CPU/allocation profili. Ham müşteri verileri/log içerikleri tutulmaz. Timer claim lag ile timer completion lag ayrı raporlanır.
2. **En büyük yapısal aday:** Bütçeli kısa adım ilerletme. Her node için scheduler'a dönmeyi azaltır. Özellikle rollback, crash, global exit, pause ve simulator parity testleri geçmeden yayınlanmaz.
3. **Daha küçük bağımsız aday:** Eşik + maksimum bekleme süresiyle refill. Aynı 8 runtime/4 delivery slotu ve aynı pool bütçeleri korunarak ölçülür. Yavaş müşteri/provider'ın boş slotları durdurmama testleri korunur.
4. **Sonra kaynak paylaşımı:** Timer ve delivery fairness. Toplam throughput artarken timer p99 veya en eski delivery yaşı bozuluyorsa ayar kabul edilmez.
5. **İhtiyaç görülürse:** Notification + fallback, sınırlı immutable-version cache ve geçmiş veri dağılımına göre bakım/index düzenlemeleri. Yeni indeksin yazma maliyeti de karşılaştırılır.

Her değişiklik tek başına ölçülmeli; normal 10.000 eşzamanlı timer, daha dağınık gerçekçi deadline'lar, yavaş provider ve aynı müşteriye gelen purchase/event yarışları birlikte doğrulanmalı. Sıfır duplicate/failed logical action, güvenli restart ve version isolation ön koşuldur. Kazançlar benchmark yapılmadan sayısallaştırılmaz.

**Sonuç:** Bizim mimariye en yakın kazanımlar bounded adım ilerletme, kapasiteye göre toplu claim ve kaynak fairness'idir. Motoru Temporal'a taşımak veya yalnızca pool'u büyütmek için mevcut ölçümler yeterli gerekçe sunmuyor. Bu tur araştırma/dokümantasyon turudur; production runtime değiştirilmedi.
